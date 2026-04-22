package ipguard

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// mockIPExtractor returns an IPExtractor function that always returns the given IP.
func mockIPExtractor(ip string) func(c fiber.Ctx) string {
	return func(_ fiber.Ctx) string {
		return ip
	}
}

// go test -run TestIPGuard_AllowedIP
func TestIPGuard_AllowedIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		allowedIPs     []string
		clientIP       string
		expectedStatus int
	}{
		{
			name:           "allowed single IPv4",
			allowedIPs:     []string{"192.168.1.1"},
			clientIP:       "192.168.1.1",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "blocked single IPv4",
			allowedIPs:     []string{"192.168.1.1"},
			clientIP:       "10.0.0.1",
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "allowed IPv4 in CIDR range",
			allowedIPs:     []string{"192.168.1.0/24"},
			clientIP:       "192.168.1.100",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "blocked IPv4 not in CIDR range",
			allowedIPs:     []string{"192.168.1.0/24"},
			clientIP:       "192.168.2.1",
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "allowed single IPv6",
			allowedIPs:     []string{"::1"},
			clientIP:       "::1",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "blocked IPv6 not in list",
			allowedIPs:     []string{"::1"},
			clientIP:       "::2",
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "allowed IPv6 in CIDR range",
			allowedIPs:     []string{"2001:db8::/32"},
			clientIP:       "2001:db8::1",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "blocked IPv6 not in CIDR range",
			allowedIPs:     []string{"2001:db8::/32"},
			clientIP:       "2001:db9::1",
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "mixed IPv4 and IPv6 CIDRs - allowed IPv4",
			allowedIPs:     []string{"10.0.0.0/8", "2001:db8::/32"},
			clientIP:       "10.5.5.5",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "mixed IPv4 and IPv6 CIDRs - allowed IPv6",
			allowedIPs:     []string{"10.0.0.0/8", "2001:db8::/32"},
			clientIP:       "2001:db8::100",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "multiple single IPs - allowed",
			allowedIPs:     []string{"1.2.3.4", "5.6.7.8"},
			clientIP:       "5.6.7.8",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "multiple single IPs - blocked",
			allowedIPs:     []string{"1.2.3.4", "5.6.7.8"},
			clientIP:       "9.9.9.9",
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "IPv4 loopback",
			allowedIPs:     []string{"127.0.0.0/8"},
			clientIP:       "127.0.0.1",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "large CIDR range /16",
			allowedIPs:     []string{"172.16.0.0/16"},
			clientIP:       "172.16.255.255",
			expectedStatus: fiber.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{
				AllowedIPs:  tt.allowedIPs,
				IPExtractor: mockIPExtractor(tt.clientIP),
			}))

			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("OK")
			})

			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

// go test -run TestIPGuard_ExcludedPaths
func TestIPGuard_ExcludedPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		path           string
		excludedPaths  []string
		clientIP       string
		allowedIPs     []string
		expectedStatus int
	}{
		{
			name:           "excluded path bypasses check",
			path:           "/health",
			excludedPaths:  []string{"/health"},
			clientIP:       "10.0.0.1",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "excluded path prefix matches subpath",
			path:           "/health/live",
			excludedPaths:  []string{"/health"},
			clientIP:       "10.0.0.1",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "non-excluded path is blocked",
			path:           "/api/data",
			excludedPaths:  []string{"/health", "/public"},
			clientIP:       "10.0.0.1",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "multiple excluded paths - match second",
			path:           "/public/assets",
			excludedPaths:  []string{"/health", "/public"},
			clientIP:       "10.0.0.1",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "no excluded paths - blocked",
			path:           "/api",
			excludedPaths:  []string{},
			clientIP:       "10.0.0.1",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "prefix match includes longer paths sharing the prefix",
			path:           "/healthcheck",
			excludedPaths:  []string{"/health"},
			clientIP:       "10.0.0.1",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusOK, // HasPrefix("/healthcheck", "/health") == true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{
				AllowedIPs:    tt.allowedIPs,
				ExcludedPaths: tt.excludedPaths,
				IPExtractor:   mockIPExtractor(tt.clientIP),
			}))

			app.Get("/*", func(c fiber.Ctx) error {
				return c.SendString("OK")
			})

			req := httptest.NewRequest(fiber.MethodGet, tt.path, http.NoBody)

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

// go test -run TestIPGuard_ExcludedExactPaths
func TestIPGuard_ExcludedExactPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		path               string
		excludedExactPaths []string
		clientIP           string
		allowedIPs         []string
		expectedStatus     int
	}{
		{
			name:               "exact match bypasses check",
			path:               "/health",
			excludedExactPaths: []string{"/health"},
			clientIP:           "10.0.0.1",
			allowedIPs:         []string{"192.168.1.1"},
			expectedStatus:     fiber.StatusOK,
		},
		{
			name:               "exact path does not match longer path",
			path:               "/healthcheck",
			excludedExactPaths: []string{"/health"},
			clientIP:           "10.0.0.1",
			allowedIPs:         []string{"192.168.1.1"},
			expectedStatus:     fiber.StatusForbidden,
		},
		{
			name:               "exact path does not match subpath",
			path:               "/health/live",
			excludedExactPaths: []string{"/health"},
			clientIP:           "10.0.0.1",
			allowedIPs:         []string{"192.168.1.1"},
			expectedStatus:     fiber.StatusForbidden,
		},
		{
			name:               "multiple exact paths - match first",
			path:               "/health",
			excludedExactPaths: []string{"/health", "/public"},
			clientIP:           "10.0.0.1",
			allowedIPs:         []string{"192.168.1.1"},
			expectedStatus:     fiber.StatusOK,
		},
		{
			name:               "multiple exact paths - match second",
			path:               "/public",
			excludedExactPaths: []string{"/health", "/public"},
			clientIP:           "10.0.0.1",
			allowedIPs:         []string{"192.168.1.1"},
			expectedStatus:     fiber.StatusOK,
		},
		{
			name:               "no exact path match - blocked",
			path:               "/api",
			excludedExactPaths: []string{"/health", "/public"},
			clientIP:           "10.0.0.1",
			allowedIPs:         []string{"192.168.1.1"},
			expectedStatus:     fiber.StatusForbidden,
		},
		{
			name:               "empty exact paths list - blocked",
			path:               "/health",
			excludedExactPaths: []string{},
			clientIP:           "10.0.0.1",
			allowedIPs:         []string{"192.168.1.1"},
			expectedStatus:     fiber.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{
				AllowedIPs:         tt.allowedIPs,
				ExcludedExactPaths: tt.excludedExactPaths,
				IPExtractor:        mockIPExtractor(tt.clientIP),
			}))

			app.Get("/*", func(c fiber.Ctx) error {
				return c.SendString("OK")
			})

			req := httptest.NewRequest(fiber.MethodGet, tt.path, http.NoBody)

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

// go test -run TestIPGuard_ExcludedExactAndPrefixCombined
func TestIPGuard_ExcludedExactAndPrefixCombined(t *testing.T) {
	t.Parallel()

	t.Run("exact match takes priority over prefix", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:         []string{"192.168.1.1"},
			ExcludedExactPaths: []string{"/health"},
			ExcludedPaths:      []string{"/api"},
			IPExtractor:        mockIPExtractor("10.0.0.1"),
		}))

		app.Get("/*", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		// Exact match on /health
		req := httptest.NewRequest(fiber.MethodGet, "/health", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		// Prefix match on /api/users
		req = httptest.NewRequest(fiber.MethodGet, "/api/users", http.NoBody)
		resp, err = app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		// No match on /other
		req = httptest.NewRequest(fiber.MethodGet, "/other", http.NoBody)
		resp, err = app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

		// Exact /health does NOT match /healthcheck
		req = httptest.NewRequest(fiber.MethodGet, "/healthcheck", http.NoBody)
		resp, err = app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	})
}

// go test -run TestIPGuard_XForwardedFor
func TestIPGuard_XForwardedFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		useXForwardedFor bool
		xffHeader        string
		remoteIP         string
		allowedIPs       []string
		expectedStatus   int
	}{
		{
			name:             "XFF enabled - first IP in header matches whitelist",
			useXForwardedFor: true,
			xffHeader:        "192.168.1.1, 10.0.0.1",
			remoteIP:         "10.0.0.1",
			allowedIPs:       []string{"192.168.1.1"},
			expectedStatus:   fiber.StatusOK,
		},
		{
			name:             "XFF enabled - first IP in header not in whitelist",
			useXForwardedFor: true,
			xffHeader:        "9.9.9.9, 10.0.0.1",
			remoteIP:         "10.0.0.1",
			allowedIPs:       []string{"192.168.1.1"},
			expectedStatus:   fiber.StatusForbidden,
		},
		{
			name:             "XFF disabled - ignores header even if IP matches",
			useXForwardedFor: false,
			xffHeader:        "192.168.1.1",
			remoteIP:         "10.0.0.1",
			allowedIPs:       []string{"192.168.1.1"},
			expectedStatus:   fiber.StatusForbidden,
		},
		{
			name:             "XFF enabled - single IP in header",
			useXForwardedFor: true,
			xffHeader:        "192.168.1.1",
			remoteIP:         "10.0.0.1",
			allowedIPs:       []string{"192.168.1.1"},
			expectedStatus:   fiber.StatusOK,
		},
		{
			name:             "XFF enabled - IPv6 in header",
			useXForwardedFor: true,
			xffHeader:        "2001:db8::1",
			remoteIP:         "10.0.0.1",
			allowedIPs:       []string{"2001:db8::1"},
			expectedStatus:   fiber.StatusOK,
		},
		{
			name:             "XFF enabled - IPv6 CIDR match in header",
			useXForwardedFor: true,
			xffHeader:        "2001:db8::100, ::1",
			remoteIP:         "10.0.0.1",
			allowedIPs:       []string{"2001:db8::/32"},
			expectedStatus:   fiber.StatusOK,
		},
		{
			name:             "XFF enabled - empty header falls back to c.IP() which is 0.0.0.0 in test",
			useXForwardedFor: true,
			xffHeader:        "",
			remoteIP:         "0.0.0.0",
			allowedIPs:       []string{"0.0.0.0"},
			expectedStatus:   fiber.StatusOK,
		},
		{
			name:             "XFF enabled - empty header falls back to c.IP() which may not be whitelisted",
			useXForwardedFor: true,
			xffHeader:        "",
			remoteIP:         "0.0.0.0",
			allowedIPs:       []string{"192.168.1.1"},
			expectedStatus:   fiber.StatusForbidden,
		},
		{
			name:             "XFF enabled - header with spaces",
			useXForwardedFor: true,
			xffHeader:        "  192.168.1.1  ,  10.0.0.1",
			remoteIP:         "10.0.0.1",
			allowedIPs:       []string{"192.168.1.1"},
			expectedStatus:   fiber.StatusOK,
		},
		{
			name:             "XFF enabled - invalid first IP skipped, second valid IP used",
			useXForwardedFor: true,
			xffHeader:        "not-an-ip, 192.168.1.1",
			remoteIP:         "10.0.0.1",
			allowedIPs:       []string{"192.168.1.1"},
			expectedStatus:   fiber.StatusOK,
		},
		{
			name:             "XFF enabled - all invalid IPs in header fall back to c.IP()",
			useXForwardedFor: true,
			xffHeader:        "not-an-ip, another-bad-ip",
			remoteIP:         "0.0.0.0",
			allowedIPs:       []string{"0.0.0.0"},
			expectedStatus:   fiber.StatusOK,
		},
		{
			name:             "XFF enabled - all invalid IPs in header fall back to c.IP() which may not be whitelisted",
			useXForwardedFor: true,
			xffHeader:        "not-an-ip, another-bad-ip",
			remoteIP:         "0.0.0.0",
			allowedIPs:       []string{"192.168.1.1"},
			expectedStatus:   fiber.StatusForbidden,
		},
		{
			name:             "XFF enabled - malicious value in header skipped",
			useXForwardedFor: true,
			xffHeader:        "<script>alert(1)</script>, 192.168.1.1",
			remoteIP:         "10.0.0.1",
			allowedIPs:       []string{"192.168.1.1"},
			expectedStatus:   fiber.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{TrustProxy: true, ProxyHeader: fiber.HeaderXForwardedFor})
			app.Use(New(Config{
				AllowedIPs:       tt.allowedIPs,
				UseXForwardedFor: tt.useXForwardedFor,
			}))

			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("OK")
			})

			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			if tt.xffHeader != "" {
				req.Header.Set(fiber.HeaderXForwardedFor, tt.xffHeader)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

// go test -run TestIPGuard_ForbiddenHandler
func TestIPGuard_ForbiddenHandler(t *testing.T) {
	t.Parallel()

	t.Run("default forbidden handler returns 403", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("10.0.0.1"),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	})

	t.Run("custom forbidden handler returns custom response", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("10.0.0.1"),
			ForbiddenHandler: func(c fiber.Ctx) error {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "access denied",
				})
			},
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "access denied")
	})
}

// go test -run TestIPGuard_Next
func TestIPGuard_Next(t *testing.T) {
	t.Parallel()

	t.Run("skip middleware when Next returns true", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			Next: func(_ fiber.Ctx) bool {
				return true
			},
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("10.0.0.1"),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("execute middleware when Next returns false", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			Next: func(_ fiber.Ctx) bool {
				return false
			},
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("10.0.0.1"),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	})
}

// go test -run TestIPGuard_EmptyWhitelist
func TestIPGuard_EmptyWhitelist(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		AllowedIPs:  []string{},
		IPExtractor: mockIPExtractor("192.168.1.1"),
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

// go test -run TestIPGuard_EmptyClientIP
func TestIPGuard_EmptyClientIP(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		AllowedIPs:  []string{"192.168.1.1"},
		IPExtractor: mockIPExtractor(""),
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

// go test -run TestIPGuard_InvalidClientIP
func TestIPGuard_InvalidClientIP(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		AllowedIPs:  []string{"192.168.1.1"},
		IPExtractor: mockIPExtractor("not-an-ip"),
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

// go test -run TestIPGuard_InvalidAllowedIP
func TestIPGuard_InvalidAllowedIP(t *testing.T) {
	t.Parallel()

	t.Run("panic message includes index and value", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "ipguard: allowedIPs[0]: invalid IP/CIDR \"not-an-ip\"", func() {
			New(Config{
				AllowedIPs: []string{"not-an-ip"},
			})
		})
	})

	t.Run("panic message includes correct index with multiple entries", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "ipguard: allowedIPs[2]: invalid IP/CIDR \"bad-ip\"", func() {
			New(Config{
				AllowedIPs: []string{"192.168.1.1", "10.0.0.0/24", "bad-ip"},
			})
		})
	})
}

// go test -run TestIPGuard_ParseAllowedIPs
func TestIPGuard_ParseAllowedIPs(t *testing.T) {
	t.Parallel()

	t.Run("valid single IPs and CIDRs", func(t *testing.T) {
		t.Parallel()

		ips, nets, err := parseAllowedIPs([]string{
			"192.168.1.1",
			"10.0.0.0/24",
			"::1",
			"2001:db8::/32",
		})
		require.NoError(t, err)
		require.Len(t, ips, 2)
		require.Len(t, nets, 2)
	})

	t.Run("invalid entry includes index and value", func(t *testing.T) {
		t.Parallel()

		_, _, err := parseAllowedIPs([]string{"invalid"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "allowedIPs[0]")
		require.Contains(t, err.Error(), "invalid")
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		ips, nets, err := parseAllowedIPs([]string{})
		require.NoError(t, err)
		require.Empty(t, ips)
		require.Empty(t, nets)
	})
}

// go test -run TestIPGuard_IsIPAllowed
func TestIPGuard_IsIPAllowed(t *testing.T) {
	t.Parallel()

	ips, nets, err := parseAllowedIPs([]string{"192.168.1.1", "10.0.0.0/24", "::1", "2001:db8::/32"})
	require.NoError(t, err)

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"exact IPv4 match", "192.168.1.1", true},
		{"no IPv4 match", "192.168.1.2", false},
		{"IPv4 in CIDR range", "10.0.0.50", true},
		{"IPv4 not in CIDR range", "10.1.0.1", false},
		{"exact IPv6 match", "::1", true},
		{"IPv6 not in list", "::2", false},
		{"IPv6 in CIDR range", "2001:db8::1", true},
		{"IPv6 not in CIDR range", "2001:db9::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ip := net.ParseIP(tt.ip)
			require.NotNil(t, ip)
			require.Equal(t, tt.expected, isIPAllowed(ip, ips, nets))
		})
	}
}

// go test -run TestIPGuard_XRealIP
func TestIPGuard_XRealIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		useXRealIP     bool
		xriHeader      string
		remoteIP       string
		allowedIPs     []string
		expectedStatus int
	}{
		{
			name:           "X-Real-IP enabled - header IP matches whitelist",
			useXRealIP:     true,
			xriHeader:      "192.168.1.1",
			remoteIP:       "10.0.0.1",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "X-Real-IP enabled - header IP not in whitelist",
			useXRealIP:     true,
			xriHeader:      "9.9.9.9",
			remoteIP:       "10.0.0.1",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "X-Real-IP disabled - ignores header even if IP matches",
			useXRealIP:     false,
			xriHeader:      "192.168.1.1",
			remoteIP:       "0.0.0.0",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "X-Real-IP enabled - IPv6 in header",
			useXRealIP:     true,
			xriHeader:      "2001:db8::1",
			remoteIP:       "10.0.0.1",
			allowedIPs:     []string{"2001:db8::1"},
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "X-Real-IP enabled - empty header falls back to c.IP()",
			useXRealIP:     true,
			xriHeader:      "",
			remoteIP:       "0.0.0.0",
			allowedIPs:     []string{"0.0.0.0"},
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "X-Real-IP enabled - invalid IP in header falls back to c.IP()",
			useXRealIP:     true,
			xriHeader:      "not-an-ip",
			remoteIP:       "0.0.0.0",
			allowedIPs:     []string{"0.0.0.0"},
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "X-Real-IP enabled - invalid IP in header not whitelisted",
			useXRealIP:     true,
			xriHeader:      "not-an-ip",
			remoteIP:       "0.0.0.0",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name:           "X-Real-IP enabled - header with spaces",
			useXRealIP:     true,
			xriHeader:      "  192.168.1.1  ",
			remoteIP:       "10.0.0.1",
			allowedIPs:     []string{"192.168.1.1"},
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "X-Real-IP enabled - IPv6 CIDR match in header",
			useXRealIP:     true,
			xriHeader:      "2001:db8::100",
			remoteIP:       "10.0.0.1",
			allowedIPs:     []string{"2001:db8::/32"},
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "X-Real-IP enabled - malicious value in header skipped",
			useXRealIP:     true,
			xriHeader:      "<script>alert(1)</script>",
			remoteIP:       "0.0.0.0",
			allowedIPs:     []string{"0.0.0.0"},
			expectedStatus: fiber.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{TrustProxy: true})
			app.Use(New(Config{
				AllowedIPs:  tt.allowedIPs,
				UseXRealIP:  tt.useXRealIP,
				IPExtractor: nil,
			}))

			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("OK")
			})

			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			if tt.xriHeader != "" {
				req.Header.Set("X-Real-IP", tt.xriHeader)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

// go test -run TestIPGuard_XForwardedForPriorityOverXRealIP
func TestIPGuard_XForwardedForPriorityOverXRealIP(t *testing.T) {
	t.Parallel()

	t.Run("XFF takes priority over X-Real-IP when both enabled", func(t *testing.T) {
		t.Parallel()

		app := fiber.New(fiber.Config{TrustProxy: true, ProxyHeader: fiber.HeaderXForwardedFor})
		app.Use(New(Config{
			AllowedIPs:       []string{"192.168.1.1"},
			UseXForwardedFor: true,
			UseXRealIP:       true,
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderXForwardedFor, "192.168.1.1")
		req.Header.Set("X-Real-IP", "9.9.9.9")

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("X-Real-IP is used when XFF is enabled but header is empty", func(t *testing.T) {
		t.Parallel()

		app := fiber.New(fiber.Config{TrustProxy: true, ProxyHeader: fiber.HeaderXForwardedFor})
		app.Use(New(Config{
			AllowedIPs:       []string{"192.168.1.1"},
			UseXForwardedFor: true,
			UseXRealIP:       true,
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Real-IP", "192.168.1.1")

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
}

// go test -run TestIPGuard_IPExtractorOverridesXFF
func TestIPGuard_IPExtractorOverridesXFF(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{TrustProxy: true, ProxyHeader: fiber.HeaderXForwardedFor})
	app.Use(New(Config{
		AllowedIPs:       []string{"10.0.0.1"},
		UseXForwardedFor: true,
		IPExtractor:      mockIPExtractor("10.0.0.1"),
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Even though XFF header says 9.9.9.9, IPExtractor takes priority
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderXForwardedFor, "9.9.9.9")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// go test -run TestIPGuard_NormalizeIPv4MappedIPv6
func TestIPGuard_NormalizeIPv4MappedIPv6(t *testing.T) {
	t.Parallel()

	t.Run("IPv4-mapped IPv6 matches plain IPv4 in whitelist", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("::ffff:192.168.1.1"),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("IPv4-mapped IPv6 matches IPv4 CIDR", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"192.168.1.0/24"},
			IPExtractor: mockIPExtractor("::ffff:192.168.1.100"),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("plain IPv4 still works", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("192.168.1.1"),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("pure IPv6 still works", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"2001:db8::1"},
			IPExtractor: mockIPExtractor("2001:db8::1"),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("IPv4-mapped IPv6 blocked when not in whitelist", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("::ffff:10.0.0.1"),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	})

	t.Run("IPv4-mapped IPv6 loopback matches 127.0.0.1", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"127.0.0.0/8"},
			IPExtractor: mockIPExtractor("::ffff:127.0.0.1"),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("default config has NormalizeIPv4MappedIPv6 enabled", func(t *testing.T) {
		t.Parallel()

		cfg := configDefault()
		require.NotNil(t, cfg.NormalizeIPv4MappedIPv6)
		require.True(t, *cfg.NormalizeIPv4MappedIPv6)
	})

	t.Run("NormalizeIPv4MappedIPv6 can be explicitly disabled", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:              []string{"192.168.1.1"},
			IPExtractor:             mockIPExtractor("::ffff:192.168.1.1"),
			NormalizeIPv4MappedIPv6: boolPtr(false),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		// Note: Go's net.ParseIP represents "::ffff:192.168.1.1" and "192.168.1.1"
		// identically as 16-byte IPv4-mapped IPv6 addresses. Even with normalization
		// disabled, Go's Equal() and Contains() still treat them as equivalent.
		// Disabling normalization only prevents the 16→4 byte conversion, but matching
		// still works due to Go's built-in equivalence handling.
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
}

// go test -run TestIPGuard_ClientIPFromContext
func TestIPGuard_ClientIPFromContext(t *testing.T) {
	t.Parallel()

	t.Run("client IP stored in context with default key", func(t *testing.T) {
		t.Parallel()

		var resolvedIP string
		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("192.168.1.1"),
		}))
		app.Get("/", func(c fiber.Ctx) error {
			resolvedIP = ClientIPFromContext(c)
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.Equal(t, "192.168.1.1", resolvedIP)
	})

	t.Run("client IP stored with IPv6", func(t *testing.T) {
		t.Parallel()

		var resolvedIP string
		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"2001:db8::1"},
			IPExtractor: mockIPExtractor("2001:db8::1"),
		}))
		app.Get("/", func(c fiber.Ctx) error {
			resolvedIP = ClientIPFromContext(c)
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.Equal(t, "2001:db8::1", resolvedIP)
	})

	t.Run("IPv4-mapped IPv6 normalized in context", func(t *testing.T) {
		t.Parallel()

		var resolvedIP string
		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("::ffff:192.168.1.1"),
		}))
		app.Get("/", func(c fiber.Ctx) error {
			resolvedIP = ClientIPFromContext(c)
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.Equal(t, "192.168.1.1", resolvedIP)
	})

	t.Run("client IP available even when request is blocked", func(t *testing.T) {
		t.Parallel()

		var resolvedIP string
		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("10.0.0.1"),
			ForbiddenHandler: func(c fiber.Ctx) error {
				resolvedIP = ClientIPFromContext(c)
				return c.SendStatus(fiber.StatusForbidden)
			},
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
		require.Equal(t, "10.0.0.1", resolvedIP)
	})

	t.Run("custom context key - stored under both default and custom keys", func(t *testing.T) {
		t.Parallel()

		var customKeyIP, defaultKeyIP string
		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:  []string{"192.168.1.1"},
			IPExtractor: mockIPExtractor("192.168.1.1"),
			ContextKey:  "my-custom-ip",
		}))
		app.Get("/", func(c fiber.Ctx) error {
			customKeyIP, _ = c.Locals("my-custom-ip").(string)
			defaultKeyIP = ClientIPFromContext(c)
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.Equal(t, "192.168.1.1", customKeyIP)
		require.Equal(t, "192.168.1.1", defaultKeyIP)
	})

	t.Run("DisableContextStorage prevents storing in context", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			AllowedIPs:            []string{"192.168.1.1"},
			IPExtractor:           mockIPExtractor("192.168.1.1"),
			DisableContextStorage: true,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			// ClientIPFromContext should return empty string when storage is disabled
			ip := ClientIPFromContext(c)
			if ip != "" {
				return c.Status(500).SendString("unexpected IP: " + ip)
			}
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("ClientIPFromContext returns empty when middleware not used", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Get("/", func(c fiber.Ctx) error {
			ip := ClientIPFromContext(c)
			if ip != "" {
				return c.Status(500).SendString("unexpected IP: " + ip)
			}
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("client IP from XFF header stored in context", func(t *testing.T) {
		t.Parallel()

		var resolvedIP string
		app := fiber.New(fiber.Config{TrustProxy: true, ProxyHeader: fiber.HeaderXForwardedFor})
		app.Use(New(Config{
			AllowedIPs:       []string{"192.168.1.1"},
			UseXForwardedFor: true,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			resolvedIP = ClientIPFromContext(c)
			return c.SendString("OK")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderXForwardedFor, "192.168.1.1")

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.Equal(t, "192.168.1.1", resolvedIP)
	})
}
