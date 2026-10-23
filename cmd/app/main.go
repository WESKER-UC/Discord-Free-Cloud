package main

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"discord-free-cloud/frontend"
	"discord-free-cloud/internal/catalog"
	"discord-free-cloud/internal/crypto"
	"discord-free-cloud/internal/db"
	"discord-free-cloud/internal/discord"
	"discord-free-cloud/internal/downloader"
	"discord-free-cloud/internal/server"
	"discord-free-cloud/internal/syswin"
	"discord-free-cloud/internal/uploader"

	"golang.org/x/term"
)

func isConsole(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

func readPasswordConsole(prompt string) (string, error) {
	fmt.Print(prompt)
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println()
	return string(password), nil
}

func main() {
	portFlag := flag.Int("port", envInt("PORT", 8080), "web server port")
	hostFlag := flag.String("host", envString("HOST", "127.0.0.1"), "web server bind address")
	dbPathFlag := flag.String("db", os.Getenv("DB_PATH"), "path to storage database file")
	noBrowserFlag := flag.Bool("no-browser", envBool("NO_BROWSER"), "skip opening browser")
	noPromptFlag := flag.Bool("no-prompt", envBool("NO_PROMPT"), "skip console password prompt")
	passwordFlag := flag.String("password", os.Getenv("MASTER_PASSWORD"), "master password flag")
	chunkSizeFlag := flag.Int("chunk-size", 8*1024*1024, "part size in bytes")
	workersFlag := flag.Int("workers", 6, "upload worker count")
	flag.Parse()

	cloudMode := envBool("CLOUD_MODE")
	appUsername := envString("APP_USERNAME", "admin")
	appPassword := os.Getenv("APP_PASSWORD")
	if cloudMode && (appPassword == "" || len(appPassword) < 16) {
		log.Fatal("CLOUD_MODE requires APP_PASSWORD to be set to at least 16 characters")
	}
	if err := validateBindSecurity(*hostFlag, appPassword); err != nil {
		log.Fatal(err)
	}

	dbPath := *dbPathFlag
	if dbPath == "" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			dbPath = filepath.Join(appData, "DiscordFreeCloud", "drive.db")
			oldPath := filepath.Join(appData, "DiscordStorageEngine", "drive.db")
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				if _, err := os.Stat(oldPath); err == nil {
					_ = os.MkdirAll(filepath.Dir(dbPath), 0755)
					_ = os.Rename(oldPath, dbPath)
				}
			}
		} else {
			dbPath = filepath.Join("data", "drive.db")
		}
	}

	fmt.Println("Discord Free Cloud by zyrexdz")
	fmt.Printf("Database: %s\n", dbPath)

	database, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("could not start database: %v", err)
	}
	defer database.Close()

	if err := database.ResetIncomplete(); err != nil {
		log.Printf("could not reset unfinished parts: %v", err)
	}

	settings, _ := database.GetAllSettings()
	botToken := settings["bot_token"]
	chunkSize := *chunkSizeFlag
	if cs, ok := settings["chunk_size_bytes"]; ok {
		if val, err := strconv.Atoi(cs); err == nil && val > 0 {
			chunkSize = val
		}
	}

	workers := *workersFlag
	if w, ok := settings["worker_count"]; ok {
		if val, err := strconv.Atoi(w); err == nil && val > 0 {
			workers = val
		}
	}

	passHash := settings["master_pass_hash"]
	saltHex := settings["master_salt_hex"]
	var salt []byte
	if saltHex != "" {
		salt, _ = hex.DecodeString(saltHex)
	}

	var masterKey []byte

	if *passwordFlag != "" {
		if len(salt) == 0 {
			salt, _ = crypto.GenerateSalt()
			_ = database.SetSetting("master_salt_hex", hex.EncodeToString(salt))
		}
		key := crypto.DeriveKey(*passwordFlag, salt)
		computedHash := crypto.ComputeSHA256(key)
		if passHash != "" && passHash != computedHash {
			log.Fatalf("incorrect encryption password")
		}
		if passHash == "" {
			_ = database.SetSetting("master_pass_hash", computedHash)
		}
		masterKey = key
		fmt.Println("Password verified. Drive unlocked.")
	} else if !*noPromptFlag && isConsole(os.Stdin.Fd()) {
		if passHash != "" {
			for attempts := 1; attempts <= 3; attempts++ {
				pass, err := readPasswordConsole(fmt.Sprintf("Enter password (attempt %d/3): ", attempts))
				if err != nil || pass == "" {
					fmt.Println("No password entered. Drive locked.")
					break
				}
				key := crypto.DeriveKey(pass, salt)
				if crypto.ComputeSHA256(key) == passHash {
					masterKey = key
					fmt.Println("Password accepted. Drive unlocked.")
					break
				}
				fmt.Println("Incorrect password, please try again.")
			}
		} else {
			fmt.Println("Set an encryption password for your storage:")
			pass1, err1 := readPasswordConsole("Password: ")
			if err1 == nil && pass1 != "" {
				pass2, err2 := readPasswordConsole("Confirm password: ")
				if err2 == nil && pass1 == pass2 {
					salt, _ = crypto.GenerateSalt()
					masterKey = crypto.DeriveKey(pass1, salt)
					_ = database.SetSetting("master_salt_hex", hex.EncodeToString(salt))
					_ = database.SetSetting("master_pass_hash", crypto.ComputeSHA256(masterKey))
					fmt.Println("Password saved. Drive unlocked.")
				} else {
					fmt.Println("Passwords did not match. Set password in browser.")
				}
			} else {
				fmt.Println("Skipped. Set password in browser.")
			}
		}
	}

	discordClient := discord.NewClient(botToken)
	uploaderEngine := uploader.NewEngine(database, discordClient, masterKey, chunkSize, workers)
	downloaderEngine := downloader.NewEngine(database, discordClient, masterKey, workers)
	catalogManager := catalog.New(database, discordClient, masterKey)

	srv := server.NewServer(
		database,
		discordClient,
		uploaderEngine,
		downloaderEngine,
		catalogManager,
		frontend.Assets,
	)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	targetPort := *portFlag
	maxPortTries := 10
	if os.Getenv("PORT") != "" {
		maxPortTries = 1
	}
	var listener net.Listener
	for try := 0; try < maxPortTries; try++ {
		port := targetPort + try
		addr := net.JoinHostPort(*hostFlag, strconv.Itoa(port))
		listener, err = net.Listen("tcp", addr)
		if err == nil {
			targetPort = port
			break
		}
	}
	if listener == nil {
		log.Fatalf("could not find an open port starting from %d", *portFlag)
	}

	serverURL := fmt.Sprintf("http://%s", listener.Addr().String())
	log.Printf("Server running at %s\n", serverURL)

	var handler http.Handler = mux
	if appPassword != "" {
		handler = basicAuth(handler, appUsername, appPassword)
	}
	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server stopped: %v", err)
		}
	}()

	if !*noBrowserFlag {
		go func() {
			time.Sleep(400 * time.Millisecond)
			_ = syswin.OpenBrowser(serverURL)
		}()
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	<-stopChan
	fmt.Println("\nClosing app...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(ctx)
	fmt.Println("Discord Free Cloud closed.")
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 || value > 65535 {
		return fallback
	}
	return value
}

func envBool(key string) bool {
	value, err := strconv.ParseBool(os.Getenv(key))
	return err == nil && value
}

func validateBindSecurity(host, password string) error {
	if isLoopbackHost(host) {
		return nil
	}
	if len(password) < 16 {
		return fmt.Errorf("binding to non-loopback host %q requires APP_PASSWORD to be at least 16 characters", host)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(host, ".")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func basicAuth(next http.Handler, username, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		providedUsername, providedPassword, ok := r.BasicAuth()
		usernameMatches := subtle.ConstantTimeCompare([]byte(providedUsername), []byte(username))
		passwordMatches := subtle.ConstantTimeCompare([]byte(providedPassword), []byte(password))
		if !ok || usernameMatches != 1 || passwordMatches != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Discord Free Cloud", charset="UTF-8"`)
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
