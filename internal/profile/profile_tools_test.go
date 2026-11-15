package profile

import "testing"

func TestProfileProtocolUsesActiveFeatures(t *testing.T) {
	for _, tc := range []struct{ name, config, want string }{
		{"inactive defaults", "H1=1\nH2=2-2\nS3=0\nS4=0\nJmin=10\nJc=0\nContentPaddingAddition=0 - 0", "WireGuard v1"},
		{"legacy padding", "S1=12\nS3=0\nS4=0", "AmneziaWG 1.0"},
		{"legacy signatures", "H1=90\nI1=<r 32>\nS3=0\nS4=0", "AmneziaWG 1.5"},
		{"ranged headers", "H1=90-100\nI1=<r 32>", "AmneziaWG 2.0+"},
		{"active extra padding", "S4=16", "AmneziaWG 2.0+"},
		{"ranged keepalive", "S3=12\n[Peer]\nPersistentKeepalive=20-25", "AmneziaWG 3.0"},
		{"trailer extension", "RandomTrailers=on", "AmneziaWG 3.1"},
		{"disabled cookie extension", "H1=90-100\nDisableCookies=off", "AmneziaWG 3.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Protocol("[Interface]\n" + tc.config); got != tc.want {
				t.Fatalf("profileProtocol = %q, want %q", got, tc.want)
			}
		})
	}
}
