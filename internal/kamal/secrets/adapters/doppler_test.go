package adapters

// Ports test/secrets/doppler_adapter_test.rb at Kamal 2.12.0.

import (
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

const (
	dopplerVersion = "doppler --version 2> /dev/null"
	dopplerMe      = "doppler me --json 2> /dev/null"
	dopplerThree   = `{
		"SECRET1": {"computed":"secret1", "computedVisibility":"unmasked", "note":""},
		"FSECRET1": {"computed":"fsecret1", "computedVisibility":"unmasked", "note":""},
		"FSECRET2": {"computed":"fsecret2", "computedVisibility":"unmasked", "note":""}}`
)

var dopplerThreeSecrets = []Secret{{"SECRET1", "secret1"}, {"FSECRET1", "fsecret1"}, {"FSECRET2", "fsecret2"}}

func dopplerSignedIn(command, output string) map[string]localshell.Result {
	return map[string]localshell.Result{dopplerVersion: ok(""), dopplerMe: ok(""), command: ok(output)}
}

func TestDopplerFetch(t *testing.T) {
	results := dopplerSignedIn("doppler secrets get SECRET1 FSECRET1 FSECRET2 --json -p my-project -c prd", dopplerThree)
	got, err := fetchWith(t, "doppler", results, []string{"SECRET1", "FSECRET1", "FSECRET2"}, "", "my-project/prd")
	assertSecrets(t, dopplerThreeSecrets, got, err)
}

func TestDopplerFetchHavingDopplerToken(t *testing.T) {
	env := func(name string) (string, bool) { return "dp.st.xxxxxxxxxxxxxxxxxxxxxx", name == "DOPPLER_TOKEN" }
	results := dopplerSignedIn("doppler secrets get SECRET1 FSECRET1 FSECRET2 --json ", dopplerThree)
	got, err := fetchWithEnv(t, "doppler", results, env, []string{"SECRET1", "FSECRET1", "FSECRET2"}, "", "")
	assertSecrets(t, dopplerThreeSecrets, got, err)
}

func TestDopplerFetchWithFolderInSecret(t *testing.T) {
	results := dopplerSignedIn("doppler secrets get SECRET1 FSECRET1 FSECRET2 --json -p my-project -c prd", dopplerThree)
	got, err := fetchWith(t, "doppler", results, []string{"my-project/prd/SECRET1", "my-project/prd/FSECRET1", "my-project/prd/FSECRET2"}, "", "")
	assertSecrets(t, dopplerThreeSecrets, got, err)
}

func TestDopplerFetchWithoutFrom(t *testing.T) {
	results := map[string]localshell.Result{dopplerVersion: ok(""), dopplerMe: ok("")}
	_, err := fetchWith(t, "doppler", results, []string{"FSECRET1", "FSECRET2"}, "", "")
	assertFetchError(t, "Missing project or config from '--from=project/config' option", err)
}

func TestDopplerFetchWithSignin(t *testing.T) {
	results := map[string]localshell.Result{dopplerVersion: ok(""), dopplerMe: failed, "doppler login -y": ok(""),
		"doppler secrets get SECRET1 --json -p my-project -c prd": ok(`{"SECRET1": {"computed":"secret1", "computedVisibility":"unmasked", "note":""}}`)}
	got, err := fetchWith(t, "doppler", results, []string{"SECRET1"}, "", "my-project/prd")
	assertSecrets(t, []Secret{{"SECRET1", "secret1"}}, got, err)
}

func TestDopplerFetchWithoutCLIInstalled(t *testing.T) {
	_, err := fetchWith(t, "doppler", map[string]localshell.Result{dopplerVersion: failed}, []string{"HOST", "PORT"}, "", "")
	assertFetchError(t, "Doppler CLI is not installed", err)
}

// Kamal names the prefixed secrets with Array#to_s when Doppler fails.
func TestDopplerFetchWhenTheCommandFails(t *testing.T) {
	results := map[string]localshell.Result{dopplerVersion: ok(""), dopplerMe: ok(""), "doppler secrets get SECRET1 --json -p my-project -c prd": failed}
	_, err := fetchWith(t, "doppler", results, []string{"SECRET1"}, "", "my-project/prd")
	assertFetchError(t, `Could not read ["my-project/prd/SECRET1"] from Doppler`, err)
}
