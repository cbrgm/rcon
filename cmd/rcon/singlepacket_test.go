package main

import "testing"

func TestResolveSinglePacket(t *testing.T) {
	cfg := Config{
		Default: "pz",
		Servers: map[string]ServerConfig{
			"pz":  {Host: "h", Port: 1, SinglePacket: true},
			"src": {Host: "h", Port: 2},
		},
	}

	cases := []struct {
		name  string
		flags Flags
		env   Env
		want  bool
	}{
		{"from config default server", Flags{}, Env{}, true},
		{"named non-single server", Flags{Server: "src"}, Env{}, false},
		{"flag enables even when server off", Flags{Server: "src", SinglePacket: true}, Env{}, true},
		{"env enables even when server off", Flags{Server: "src"}, Env{SinglePacket: true}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Resolve(cfg, c.flags, c.env)
			if err != nil {
				t.Fatal(err)
			}
			if got.SinglePacket != c.want {
				t.Fatalf("SinglePacket = %v, want %v", got.SinglePacket, c.want)
			}
		})
	}
}
