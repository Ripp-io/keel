package signing

// SignatureDrift captures signature verification status during drift detection.
type SignatureDrift struct {
	ManifestSigned   bool             `json:"manifest_signed"`
	ManifestVerified bool             `json:"manifest_verified"`
	ManifestKeyID    string           `json:"manifest_key_id,omitempty"`
	ManifestSignedAt string           `json:"manifest_signed_at,omitempty"`
	ManifestError    string           `json:"manifest_error,omitempty"`
	Layers           []LayerSigStatus `json:"layers,omitempty"`
}

// LayerSigStatus reports the signature status of a layer in drift output.
type LayerSigStatus struct {
	Path     string `json:"path"`
	Signed   bool   `json:"signed"`
	KeyID    string `json:"key_id,omitempty"`
	Verified bool   `json:"verified,omitempty"`
}
