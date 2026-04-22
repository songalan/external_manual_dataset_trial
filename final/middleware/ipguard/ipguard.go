package ipguard

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// New creates a new middleware handler that restricts access based on an IP whitelist.
//
// The middleware checks the client's IP address against a configured list of allowed IPs
// and CIDR ranges. Both IPv4 and IPv6 are supported. Optionally, the X-Forwarded-For
// or X-Real-IP header can be parsed to extract the client IP (use with caution behind
// a trusted proxy). Specific path prefixes can be excluded from the check.
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Parse allowed IPs into lookup structures at initialization time
	allowedIPs, allowedNets, err := parseAllowedIPs(cfg.AllowedIPs)
	if err != nil {
		panic("ipguard: " + err.Error())
	}

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Check if the path is excluded from IP checking
		path := c.Path()
		for _, exactPath := range cfg.ExcludedExactPaths {
			if path == exactPath {
				return c.Next()
			}
		}
		for _, prefix := range cfg.ExcludedPaths {
			if strings.HasPrefix(path, prefix) {
				return c.Next()
			}
		}

		// Get the client IP address
		clientIP := getClientIP(c, cfg.IPExtractor, cfg.UseXForwardedFor, cfg.UseXRealIP)
		if clientIP == "" {
			return cfg.ForbiddenHandler(c)
		}

		// Parse the client IP
		ip := net.ParseIP(clientIP)
		if ip == nil {
			return cfg.ForbiddenHandler(c)
		}

		// Normalize IPv4-mapped IPv6 addresses if enabled
		// e.g. ::ffff:192.168.1.1 → 192.168.1.1
		// Go's net.ParseIP represents both "::ffff:1.2.3.4" and "1.2.3.4" as the same
		// 16-byte IPv4-mapped IPv6 internally, so normalization converts them to the
		// canonical 4-byte form for consistent comparisons with parseAllowedIPs results.
		normalizeMapped := cfg.NormalizeIPv4MappedIPv6 != nil && *cfg.NormalizeIPv4MappedIPv6
		if normalizeMapped {
			if v4 := ip.To4(); v4 != nil {
				ip = v4
			}
		}

		// Store the resolved client IP in the context for downstream handlers.
		// The IP is always stored under the default key ("ipguard:clientIP") so that
		// ClientIPFromContext can reliably retrieve it regardless of configuration.
		// If a custom ContextKey is configured and differs from the default, the IP
		// is also stored under the custom key for direct c.Locals() access.
		if !cfg.DisableContextStorage {
			ipStr := ip.String()
			c.Locals(defaultContextKey, ipStr)
			if cfg.ContextKey != "" && cfg.ContextKey != defaultContextKey {
				c.Locals(cfg.ContextKey, ipStr)
			}
		}

		// Check if the IP is in the whitelist
		if isIPAllowed(ip, allowedIPs, allowedNets) {
			return c.Next()
		}

		// IP not in whitelist, invoke the forbidden handler
		return cfg.ForbiddenHandler(c)
	}
}

// getClientIP returns the client IP address using the configured extraction method.
func getClientIP(c fiber.Ctx, ipExtractor func(fiber.Ctx) string, useXForwardedFor, useXRealIP bool) string {
	// Use custom IP extractor if provided
	if ipExtractor != nil {
		return ipExtractor(c)
	}

	// Parse X-Forwarded-For header if enabled (takes priority over X-Real-IP)
	if useXForwardedFor {
		if xff := c.Get(fiber.HeaderXForwardedFor); xff != "" {
			// X-Forwarded-For may contain multiple IPs: client, proxy1, proxy2, ...
			// The first one is the original client IP. Validate each entry and
			// return the first valid one to prevent spoofing with malformed values.
			for _, entry := range strings.Split(xff, ",") {
				ip := utils.Trim(entry, ' ')
				if ip != "" && net.ParseIP(ip) != nil {
					return ip
				}
			}
		}
	}

	// Parse X-Real-IP header if enabled
	if useXRealIP {
		if xri := c.Get("X-Real-IP"); xri != "" {
			ip := utils.Trim(xri, ' ')
			if ip != "" && net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	return c.IP()
}

// isIPAllowed checks whether the given IP is in the allowed IPs or CIDR ranges.
func isIPAllowed(ip net.IP, allowedIPs []net.IP, allowedNets []*net.IPNet) bool {
	// Check against single IPs
	for _, allowedIP := range allowedIPs {
		if allowedIP.Equal(ip) {
			return true
		}
	}

	// Check against CIDR ranges
	for _, ipNet := range allowedNets {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// ClientIPFromContext returns the client IP resolved by the ipguard middleware
// from the Fiber context. This avoids re-parsing X-Forwarded-For, X-Real-IP, or
// other headers in downstream handlers.
//
// The IP is always stored under the default key "ipguard:clientIP" regardless of
// the ContextKey configuration, so this function works reliably even when a custom
// ContextKey is set. To access the IP via a custom key, use c.Locals(key) directly.
//
// Returns an empty string if the middleware has not set the value or
// DisableContextStorage is true.
func ClientIPFromContext(c fiber.Ctx) string {
	val, _ := c.Locals(defaultContextKey).(string)
	return val
}
