package tools

import "testing"

func TestLooksLikeDevServerCommand(t *testing.T) {
	blocked := []string{
		"npm run dev",
		"npm run dev -- --port 8080",
		"npm dev",
		"pnpm run dev",
		"yarn dev",
		"bun run start",
		"npm start",
		"npm run serve",
		"npx next dev",
		"next dev",
		"vite",
		"npx vite",
		"ng serve",
		"react-scripts start",
		"vue-cli-service serve",
		"python -m http.server 8000",
		"python3 -m SimpleHTTPServer",
		"npx serve -s dist -p 8080",
		"serve -s build",
		"flask run",
		"uvicorn app:app",
	}
	for _, c := range blocked {
		if _, ok := looksLikeDevServerCommand(c); !ok {
			t.Errorf("looksLikeDevServerCommand(%q) = not blocked, want blocked", c)
		}
	}

	allowed := []string{
		"npm test",
		"npm run build",
		"npm run lint",
		"go run .",
		"go build ./...",
		"go test ./...",
		"git status",
		"ls -la",
		"node build.js",
		"python script.py",
		"go run ./cmd/gen --out x",
		"make test",
		"wails build",
	}
	for _, c := range allowed {
		if reason, ok := looksLikeDevServerCommand(c); ok {
			t.Errorf("looksLikeDevServerCommand(%q) = blocked (%s), want not blocked", c, reason)
		}
	}
}

func TestExtractServerURLFromOutput(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "vite conflict bump",
			output: "Port 5173 is in use, trying another one...\n➜  Local:   http://localhost:5174/\n",
			want:   "http://127.0.0.1:5174",
		},
		{
			name:   "next conflict",
			output: "Port 3000 is in use, trying another one...\n▲ Next.js 14.0.0\n   - Local: http://localhost:3001\n",
			want:   "http://127.0.0.1:3001",
		},
		{
			name:   "ansi wrapped port",
			output: "VITE v6.0.2  ready in 345 ms\n➜  Local:   http://localhost:\x1b[1m5175\x1b[22m/\n",
			want:   "http://127.0.0.1:5175",
		},
		{
			name:   "no url",
			output: "Compiling...\n10 files",
			want:   "",
		},
		{
			name:   "plain port only",
			output: "Listening on port 4000\n",
			want:   "http://127.0.0.1:4000",
		},
	}

	for _, c := range cases {
		if got := extractServerURLFromOutput(c.output); got != c.want {
			t.Errorf("%s: extractServerURLFromOutput = %q, want %q", c.name, got, c.want)
		}
	}
}
