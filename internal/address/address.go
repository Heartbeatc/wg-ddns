package address

import "strings"

func CIDRIP(v string) string {
	if idx := strings.Index(v, "/"); idx >= 0 {
		return v[:idx]
	}
	return v
}

func Port(v string) string {
	idx := strings.LastIndex(v, ":")
	if idx == -1 || idx == len(v)-1 {
		return ""
	}
	return v[idx+1:]
}

func Host(v string) string {
	parts := strings.Split(v, ":")
	if len(parts) < 2 {
		return v
	}
	return strings.Join(parts[:len(parts)-1], ":")
}
