package main

import (
	"bytes"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/cliframe"
)

func TestVersionPrintsRelease(t *testing.T) {
	var out bytes.Buffer
	if err := cliframe.Execute(rootCommand(), []string{"version"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0.1.0\n" {
		t.Fatalf("version printed %q, want %q", out.String(), "0.1.0\n")
	}
}
