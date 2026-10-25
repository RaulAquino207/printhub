package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/RaulAquino207/printhub/apps/api/internal/auth"
	"github.com/RaulAquino207/printhub/apps/api/internal/store"
)

const usage = `usage:
  printhub-api                     run the API server
  printhub-api user add <username> create a user (password from the terminal or stdin)`

// runCommand handles the command-line subcommands. There is no sign-up
// screen: users are created by whoever runs the server.
func runCommand(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 3 || args[0] != "user" || args[1] != "add" {
		return errors.New(usage)
	}
	username := args[2]

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := os.MkdirAll(cfg.dataDir, 0o700); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}

	password, err := readPassword(stdin, stdout)
	if err != nil {
		return err
	}

	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(cfg.dataDir, "printhub.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	user, err := auth.NewService(db).Register(ctx, username, password)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "user %q created\n", user.Username)
	return nil
}

// readPassword asks twice without echo on a terminal, or reads one line from
// stdin otherwise (e.g. `echo "$PASSWORD" | printhub-api user add raul`).
func readPassword(stdin io.Reader, stdout io.Writer) (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(stdout, "Password: ")
		first, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stdout)
		if err != nil {
			return "", err
		}
		fmt.Fprint(stdout, "Repeat password: ")
		second, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stdout)
		if err != nil {
			return "", err
		}
		if string(first) != string(second) {
			return "", errors.New("passwords do not match")
		}
		return string(first), nil
	}

	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
