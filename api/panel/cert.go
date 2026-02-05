package panel

// CertReport contains certificate SHA256 hashes for reporting
type CertReport struct {
	NodeType          string `json:"node_type"`
	NodeID            int    `json:"node_id"`
	CertSHA256        string `json:"pinned_peer_cert_sha256"`
	PubkeySHA256      string `json:"pinned_peer_pubkey_sha256"`
}

// ReportCertificate reports the certificate and public key SHA256 hashes to the panel
func (c *Client) ReportCertificate(report *CertReport) error {
	const path = "/api/v1/server/UniProxy/cert"
	r, err := c.client.R().
		SetBody(report).
		ForceContentType("application/json").
		Post(path)
	return c.checkResponse(r, path, err)
}
