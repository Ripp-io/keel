package remote

// DisableSSRFForTest disables SSRF host validation globally. This is intended
// for use from test code in other packages that need to call FetchFile with
// httptest.NewServer URLs (which bind to 127.0.0.1). Call RestoreSSRFForTest
// in a t.Cleanup to restore the original value.
func DisableSSRFForTest() {
	skipSSRFValidation.Store(true)
}

// RestoreSSRFForTest re-enables SSRF host validation. Call this in t.Cleanup
// after DisableSSRFForTest.
func RestoreSSRFForTest() {
	skipSSRFValidation.Store(false)
}
