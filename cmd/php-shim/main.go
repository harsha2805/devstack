// Command php-shim is installed twice in <app>\shims, as php.exe and as
// composer.exe, first on PATH. It runs the PHP of the site the current folder
// belongs to, with the php.ini devstack generates for it, so `php artisan`,
// `composer install` and the like match what the site is served with. As
// composer.exe it runs devstack's pinned composer.phar with that PHP,
// downloading it the first time. See package shim.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/harsha2805/devstack/internal/appconf"
	"github.com/harsha2805/devstack/internal/shim"
)

// version is set at build time, like devstack's.
var version = "dev"

func main() {
	code, err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "devstack:", err)
	}
	os.Exit(code)
}

func run() (int, error) {
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0])))
	cwd, err := os.Getwd()
	if err != nil {
		return 1, err
	}
	settings, _ := appconf.Load()
	root := shim.FindRoot(cwd, os.Getenv("DEVSTACK_ROOT"), settings.Root, appconf.Trusted)
	if root == "" {
		return 1, errors.New("no stack is set up yet; open the devstack app first")
	}
	m, err := shim.Load(root)
	if err != nil {
		return 1, err
	}
	php, _, err := m.Pick(cwd, os.Getenv("DEVSTACK_PHP"))
	if err != nil {
		return 1, err
	}

	args := os.Args[1:]
	if name == "composer" {
		phar, err := composerPhar(root)
		if err != nil {
			return 1, err
		}
		args = append([]string{phar}, args...)
	}

	cmd := exec.Command(php.Exe, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// PHPRC rather than -c, so PHP processes the command starts (artisan
	// queue workers, Composer scripts) load the same php.ini.
	cmd.Env = setEnv(os.Environ(), "PHPRC", filepath.Dir(php.INI))
	// Ctrl+C reaches PHP directly (same console); the shim waits for it to
	// finish rather than dying first and leaving PHP behind.
	signal.Notify(make(chan os.Signal, 1), os.Interrupt)
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if err != nil {
		return 1, fmt.Errorf("running PHP %s: %w", php.Version, err)
	}
	return 0, nil
}

// composerPhar is devstack's composer.phar. The first time Composer is used,
// devstack.exe downloads and verifies it (`pkg composer`): the shim itself
// doesn't link the downloader, since every php command starts it.
func composerPhar(root string) (string, error) {
	bin := filepath.Join(root, "bin")
	if p, ok := shim.FindComposer(bin); ok {
		return p, nil
	}
	devstack, err := devstackExe()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(devstack, "--root", root, "pkg", "composer")
	cmd.Stdout, cmd.Stderr = io.Discard, os.Stderr // it prints its progress to stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("installing Composer: %w", err)
	}
	if p, ok := shim.FindComposer(bin); ok {
		return p, nil
	}
	return "", errors.New("Composer didn't install; run `devstack pkg composer` to see why")
}

// devstackExe is the devstack.exe of this shim's installation (the shims
// folder's parent), else the one on PATH: the administrators-only copy of the
// shims in Program Files has no devstack.exe beside it.
func devstackExe() (string, error) {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(filepath.Dir(exe)), "devstack.exe")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("devstack.exe"); err == nil {
		return p, nil
	}
	return "", errors.New("can't find devstack.exe to install Composer; reinstall devstack")
}

// setEnv replaces or adds key in a Windows environment, where names ignore case.
func setEnv(env []string, key, value string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if k, _, ok := strings.Cut(kv, "="); ok && strings.EqualFold(k, key) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, key+"="+value)
}
