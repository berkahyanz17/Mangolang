package scanner

// wellKnownServices maps common ports to their conventional service name.
// This is just a label for display purposes — it does NOT verify what's
// actually running on that port (that would require banner grabbing or
// protocol probing).
var wellKnownServices = map[int]string{
	20:    "ftp-data",
	21:    "ftp",
	22:    "ssh",
	23:    "telnet",
	25:    "smtp",
	53:    "dns",
	67:    "dhcp",
	80:    "http",
	110:   "pop3",
	123:   "ntp",
	135:   "msrpc",
	139:   "netbios-ssn",
	143:   "imap",
	389:   "ldap",
	443:   "https",
	445:   "microsoft-ds",
	465:   "smtps",
	587:   "smtp-submission",
	631:   "ipp",
	993:   "imaps",
	995:   "pop3s",
	1433:  "mssql",
	1521:  "oracle",
	1723:  "pptp",
	3306:  "mysql",
	3389:  "rdp",
	5432:  "postgresql",
	5900:  "vnc",
	6379:  "redis",
	8000:  "http-alt",
	8080:  "http-proxy",
	8443:  "https-alt",
	9200:  "elasticsearch",
	27017: "mongodb",
}

// ServiceName returns the conventional name for a well-known port, or
// "unknown" if it isn't in the table.
func ServiceName(port int) string {
	if name, ok := wellKnownServices[port]; ok {
		return name
	}
	return "unknown"
}
