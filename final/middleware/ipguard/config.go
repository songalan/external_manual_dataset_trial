package ipguard

import (
	"fmt"
	"net"

	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// AllowedIPs is a list of IP addresses or CIDR ranges that are allowed to access.
	// Supports both IPv4 and IPv6.
	// Examples: "192.168.1.1", "10.0.0.0/24", "::1", "2001:db8::/32"
	//
	// Required. Default: []string{}
	AllowedIPs []string

	// ForbiddenHandler is called when the IP is not in the whitelist.
	// By default, it returns a 403 Forbidden response.
	//
	// Optional. Default: func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusForbidden) }
	ForbiddenHandler fiber.Handler

	// ExcludedPaths is a list of path prefixes that should be excluded from IP checking.
	// Requests whose path starts with any of these prefixes will bypass the whitelist check.
	// For exact path matching (e.g. "/health" should not match "/healthcheck"), use
	// ExcludedExactPaths instead or append a trailing slash to the prefix (e.g. "/health/").
	// Examples: "/health", "/public"
	//
	// Optional. Default: []string{}
	ExcludedPaths []string

	// ExcludedExactPaths is a list of exact paths that should be excluded from IP checking.
	// Only requests whose path exactly matches one of these entries will bypass the whitelist check.
	// Unlike ExcludedPaths, "/health" will NOT match "/healthcheck".
	// Examples: "/health", "/public"
	//
	// Optional. Default: []string{}
	ExcludedExactPaths []string

	// IPExtractor is a function to extract the client IP from the request.
	// When set, this function is used instead of the default IP extraction logic.
	// This allows custom IP extraction for testing or special proxy setups.
	//
	// Optional. Default: nil (uses c.IP() or X-Forwarded-For based on UseXForwardedFor)
	IPExtractor func(c fiber.Ctx) string

	// UseXForwardedFor enables parsing of the X-Forwarded-For header to extract
	// the client IP address. When enabled, the middleware will check the first
	// IP address in the X-Forwarded-For header against the whitelist.
	// Make sure your application is behind a trusted proxy before enabling this.
	// This setting is ignored if IPExtractor is set.
	//
	// Optional. Default: false
	UseXForwardedFor bool

	// UseXRealIP enables parsing of the X-Real-IP header to extract
	// the client IP address. When enabled, the middleware will use the value
	// of the X-Real-IP header as the client IP if it contains a valid IP address.
	// Make sure your application is behind a trusted proxy before enabling this.
	// This setting is ignored if IPExtractor is set or UseXForwardedFor is enabled.
	//
	// Optional. Default: false
	UseXRealIP bool

	// NormalizeIPv4MappedIPv6 enables normalization of IPv4-mapped IPv6 addresses
	// (e.g. "::ffff:192.168.1.1") to their plain IPv4 form (e.g. "192.168.1.1").
	// This is useful when the server listens on an IPv6 socket and receives IPv4
	// connections, which are represented as IPv4-mapped IPv6 addresses.
	// When enabled (default), a client connecting from "::ffff:192.168.1.1" will match
	// a whitelist entry of "192.168.1.1" or "192.168.1.0/24".
	// Note: Go's net.ParseIP represents both "::ffff:1.2.3.4" and "1.2.3.4" identically
	// as 16-byte IPv4-mapped IPv6 addresses internally, so disabling this option has
	// limited practical effect — matching will still work via Go's built-in equivalence.
	//
	// Optional. Default: true
	NormalizeIPv4MappedIPv6 *bool

	// ContextKey is the key used to store the resolved client IP in c.Locals().
	// The middleware will store the client IP string (after extraction and
	// optional IPv4-mapped IPv6 normalization) in the Fiber context, making it
	// available to downstream handlers via ClientIPFromContext(c) or c.Locals(key).
	// This is useful when other handlers need access to the resolved client IP
	// (e.g. for logging, rate limiting, audit trails) without re-parsing headers.
	// Set DisableContextStorage to true to skip storing the client IP in the context.
	//
	// Optional. Default: "ipguard:clientIP"
	ContextKey string

	// DisableContextStorage disables storing the resolved client IP in the Fiber context.
	// When true, the middleware will not call c.Locals() to store the IP, and
	// ClientIPFromContext(c) will return an empty string for downstream handlers.
	// This can be useful for minimal overhead when the context value is not needed.
	//
	// Optional. Default: false
	DisableContextStorage bool
}

// defaultContextKey is the default key used to store the client IP in c.Locals().
const defaultContextKey = "ipguard:clientIP"

// ConfigDefault is the default config.
var ConfigDefault = Config{
	AllowedIPs: []string{},
	ForbiddenHandler: func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusForbidden)
	},
	ExcludedPaths:           []string{},
	ExcludedExactPaths:      []string{},
	IPExtractor:             nil,
	UseXForwardedFor:        false,
	UseXRealIP:              false,
	NormalizeIPv4MappedIPv6: boolPtr(true),
	ContextKey:              defaultContextKey,
	DisableContextStorage:   false,
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool {
	return &b
}

// Helper function to set default values.
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.ForbiddenHandler == nil {
		cfg.ForbiddenHandler = ConfigDefault.ForbiddenHandler
	}

	if cfg.AllowedIPs == nil {
		cfg.AllowedIPs = ConfigDefault.AllowedIPs
	}

	if cfg.ExcludedPaths == nil {
		cfg.ExcludedPaths = ConfigDefault.ExcludedPaths
	}

	if cfg.ExcludedExactPaths == nil {
		cfg.ExcludedExactPaths = ConfigDefault.ExcludedExactPaths
	}

	if cfg.NormalizeIPv4MappedIPv6 == nil {
		cfg.NormalizeIPv4MappedIPv6 = boolPtr(true)
	}

	return cfg
}

// parseAllowedIPs parses the allowed IPs list into separate single IPs and CIDR networks.
func parseAllowedIPs(allowedIPs []string) ([]net.IP, []*net.IPNet, error) {
	var ips []net.IP
	var nets []*net.IPNet

	for i, ipStr := range allowedIPs {
		// Try parsing as CIDR first
		if _, ipNet, err := net.ParseCIDR(ipStr); err == nil {
			nets = append(nets, ipNet)
			continue
		}

		// Try parsing as a single IP
		if ip := net.ParseIP(ipStr); ip != nil {
			ips = append(ips, ip)
			continue
		}

		// Invalid entry — include index and value for clear diagnostics
		return nil, nil, fmt.Errorf("allowedIPs[%d]: invalid IP/CIDR %q", i, ipStr)
	}

	return ips, nets, nil
}
