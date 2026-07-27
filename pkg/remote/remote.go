package remote

import "fmt"

// Resolve takes a raw layer segment and, if remote, fetches to cache.
// Returns the local directory path, the original URL (empty for local), and any error.
func Resolve(segment string, cacheDir string, offline bool) (localPath string, originalURL string, err error) {
	src, err := Parse(segment)
	if err != nil {
		return "", "", fmt.Errorf("parsing layer source %q: %w", segment, err)
	}

	switch src.Type {
	case SourceLocal:
		return segment, "", nil
	case SourceGit:
		path, err := FetchGit(src, cacheDir, offline)
		if err != nil {
			return "", "", err
		}
		return path, src.Original, nil
	case SourceArchive:
		path, err := FetchHTTP(src, cacheDir, offline)
		if err != nil {
			return "", "", err
		}
		return path, src.Original, nil
	default:
		return segment, "", nil
	}
}
