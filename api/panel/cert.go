package panel

// ReportCertificate reports the certificate SHA256 hash to the panel
func (c *Client) ReportCertificate(certSHA256 string) error {
	const path = "/api/v1/server/UniProxy/cert"
	data := map[string]string{
		"pinned_peer_cert_sha256": certSHA256,
	}
	r, err := c.client.R().
		SetBody(data).
		ForceContentType("application/json").
		Post(path)
	return c.checkResponse(r, path, err)
}
