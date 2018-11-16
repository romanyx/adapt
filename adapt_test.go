package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntegration(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get pwd: %v", err)
	}

	tests := []struct {
		name        string
		args        []string
		expect      string
		expectError string
	}{
		{
			name:   "pkg",
			args:   []string{"net/http", "Handler"},
			expect: "type handlerFunc func(http.ResponseWriter, *http.Request)\n\nfunc (f handlerFunc) ServeHTTP(rw http.ResponseWriter, r *http.Request) {\n\tf(rw, r)\n}\n",
		},
		{
			name:   "pkg return",
			args:   []string{"io", "Reader"},
			expect: "type readerFunc func([]byte) (int, error)\n\nfunc (f readerFunc) Read(p []byte) (int, error) {\n\treturn f(p)\n}\n",
		},
		{
			name:   "variadic",
			args:   []string{"variadic"},
			expect: "type variadicFunc func(...io.Reader)\n\nfunc (f variadicFunc) Func(r ...io.Reader) {\n\tf(r...)\n}\n",
		},
		{
			name:   "same names",
			args:   []string{"sameNames"},
			expect: "type sameNamesFunc func(http.Request, http.Request)\n\nfunc (f sameNamesFunc) Func(r http.Request, re http.Request) {\n\tf(r, re)\n}\n",
		},
		{
			name:        "usage",
			args:        []string{},
			expectError: "mockf [package] <interface>\nmockf generates type func to implement interface.\nExamples:\nmockf io Reader\nmockf iface\nexit status 2\n",
		},
		{
			name:        "unknow iface",
			args:        []string{"unknown"},
			expectError: "fill interface: find interface: type unknown not found in: main\nexit status 2\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := []string{"run", filepath.Join(pwd, "adapt.go")}
			args = append(args, tt.args...)
			cmd := exec.Command("go", args...)

			var buf bytes.Buffer
			cmd.Stdout = &buf
			cmd.Stderr = &buf

			if err := cmd.Run(); err != nil {
				assert.Equal(t, tt.expectError, buf.String())
				return
			}

			assert.Equal(t, tt.expect, buf.String())
		})
	}
}

func Test_generateName(t *testing.T) {
	type args struct {
		words []string
		check map[string]bool
		n     int
	}

	tests := []struct {
		name   string
		args   args
		expect string
	}{
		{
			name: "one word",
			args: args{
				words: []string{"reader"},
				check: make(map[string]bool),
				n:     1,
			},
			expect: "r",
		},
		{
			name: "same letter",
			args: args{
				words: []string{"request"},
				check: map[string]bool{
					"r": true,
				},
				n: 1,
			},
			expect: "re",
		},
		{
			name: "whole word",
			args: args{
				words: []string{"request"},
				check: map[string]bool{
					"r": true,
				},
				n: 10,
			},
			expect: "request",
		},
		{
			name: "two words",
			args: args{
				words: []string{"response", "writer"},
				check: make(map[string]bool),
				n:     1,
			},
			expect: "rw",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := generateName(tt.args.words, tt.args.check, tt.args.n)

			assert.Equal(t, tt.expect, got)
		})
	}
}
