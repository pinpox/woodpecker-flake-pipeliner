package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-ap/httpsig"
	"github.com/joho/godotenv"
	"github.com/woodpecker-ci/woodpecker/server/model"
)

type config struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type incoming struct {
	Repo          *model.Repo  `json:"repo"`
	Build         *model.Build `json:"pipeline"`
	Configuration []*config    `json:"configs"`
}

var (
	envFlakeOutput      string
	envFilterRegex      string
	envHost             string
	envPubKeyPath       string
	skipSignatureVerify bool
)

func main() {

	// Load and check configuration
	err := godotenv.Load()
	if err != nil {
		log.Printf("No loadable .env file found, you will have to provide configuration via the environment: %v", err)
	}

	// Key in format of the one fetched from http(s)://your-woodpecker-server/api/signature/public-key
	envPubKeyPath = os.Getenv("CONFIG_SERVICE_PUBLIC_KEY_FILE")
	envHost = os.Getenv("CONFIG_SERVICE_HOST")
	envFilterRegex = os.Getenv("CONFIG_SERVICE_OVERRIDE_FILTER")
	envFlakeOutput = os.Getenv("CONFIG_SERVICE_FLAKE_OUTPUT")
	skipSignatureVerify, _ = strconv.ParseBool(os.Getenv("CONFIG_SERVICE_SKIP_VERIFY"))

	if envPubKeyPath == "" && envHost == "" {
		log.Fatal("Please make sure CONFIG_SERVICE_HOST and CONFIG_SERVICE_PUBLIC_KEY_FILE are set properly")
	}

	// Serve handlers
	pipelineHandler := http.HandlerFunc(servePipeline)
	if skipSignatureVerify {
		http.Handle("/", pipelineHandler)
	} else {
		http.Handle("/", verifySignature(pipelineHandler))
	}

	log.Printf("Starting Woodpecker Config Server at: %s\n", envHost)
	log.Fatal(http.ListenAndServe(envHost, nil))
}

func servePipeline(w http.ResponseWriter, r *http.Request) {

	// Only handle POST
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Ready request body
	var req incoming
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "can't read body", http.StatusBadRequest)
		return
	}

	// Parse JSON
	err = json.Unmarshal(body, &req)
	if err != nil {
		log.Printf("Failed to parse JSON: %v", err.Error())
		http.Error(w, "Failed to parse JSON"+err.Error(), http.StatusBadRequest)
		return
	}

	// Check if repo matches the filter
	filter := regexp.MustCompile(envFilterRegex)
	if !filter.MatchString(req.Repo.Name) {
		w.WriteHeader(http.StatusNoContent) // 204 - use default config
		// No need to write a response body
		return
	}

	// Try to get pipeline. Checks are separate here, so that we don't try
	// to build anything for repos not matching the filter
	if flakePipeline, err := getPipelineFromFlake(req); err != nil {
		log.Printf("Failed to create the pipeline: %s", err)
		w.WriteHeader(http.StatusNoContent)
	} else {
		// Pipeline was build. Try to write it back
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(flakePipeline)
		if err != nil {
			log.Printf("Failed to write the pipeline: %s", err)
		}
	}
}

func verifySignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		pubKeyRaw, err := os.ReadFile(envPubKeyPath)
		if err != nil {
			log.Fatal("Failed to read public key file")
		}

		pemblock, _ := pem.Decode(pubKeyRaw)

		b, err := x509.ParsePKIXPublicKey(pemblock.Bytes)
		if err != nil {
			log.Fatal("Failed to parse public key file ", err)
		}
		pubKey, ok := b.(ed25519.PublicKey)
		if !ok {
			log.Fatal("Failed to parse public key file")
		}

		// check signature
		pubKeyID := "woodpecker-ci-plugins"

		keystore := httpsig.NewMemoryKeyStore()
		keystore.SetKey(pubKeyID, pubKey)

		verifier := httpsig.NewVerifier(keystore)
		verifier.SetRequiredHeaders([]string{"(request-target)", "date"})

		keyID, err := verifier.Verify(r)
		if err != nil {
			log.Printf("config: invalid or missing signature in http.Request")
			http.Error(w, "Invalid or Missing Signature", http.StatusBadRequest)
			return
		}

		if keyID != pubKeyID {
			log.Printf("config: invalid signature in http.Request")
			http.Error(w, "Invalid Signature", http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r) // forward the request and response to next handler.
	})
}

func runShellCmds(commands []string) ([]byte, error) {

	env := os.Environ()
	script := ""
	for _, cmd := range commands {
		script += fmt.Sprintf("%s\n", cmd)
	}
	script = strings.TrimSpace(script)

	log.Println("Script: ", script)

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = env

	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()
	if err != nil {
		log.Println(errb.String())
		return nil, err
	}

	return outb.Bytes(), nil

}

func getPipelineFromFlake(req incoming) ([]byte, error) {

	var output []byte
	var err error

	log.Println("Running Pre-commands")
    commands := strings.Split(os.Getenv("PRE_CMD"), "\n")

	if output, err = runShellCmds(commands); err != nil {
		return nil, err
	}
	log.Println("Pre-Commands output:\n", string(output))

	// Construct flake url and build
	buildURL := fmt.Sprintf(
		"'git+%s?ref=%s&rev=%s#%s'",
		req.Repo.Link,
		req.Build.Ref,
		req.Build.Commit,
		envFlakeOutput,
	)

	log.Println("Constructed flake build URL:", buildURL)

	// Run
	commands = []string{fmt.Sprintf("nix build --print-out-paths %s", buildURL)}
	if output, err = runShellCmds(commands); err != nil {
		return nil, err
	}

	// Trim whitespace and newlines
	nixStorePath := strings.TrimSpace(string(output))
	log.Println("Got nix-store path:", nixStorePath)

	b, err := os.ReadFile(nixStorePath)
	if err != nil {
		return nil, err
	}

	return b, nil

}
