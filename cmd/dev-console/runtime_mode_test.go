package main

import "testing"

func TestSelectRuntimeMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   *serverConfig
		isTTY bool
		want  runtimeMode
	}{
		{
			name:  "bridge flag wins",
			cfg:   &serverConfig{bridgeMode: true},
			isTTY: true,
			want:  modeBridge,
		},
		{
			name:  "daemon flag wins",
			cfg:   &serverConfig{daemonMode: true},
			isTTY: false,
			want:  modeDaemon,
		},
		{
			name:  "tty defaults to bridge mode",
			cfg:   &serverConfig{},
			isTTY: true,
			want:  modeBridge,
		},
		{
			name:  "non-tty defaults to bridge mode",
			cfg:   &serverConfig{},
			isTTY: false,
			want:  modeBridge,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := selectRuntimeMode(tt.cfg, tt.isTTY)
			if got != tt.want {
				t.Fatalf("selectRuntimeMode() = %q, want %q", got, tt.want)
			}
		})
	}
}
