package scanner

const sampleThresholdBytes = 50 * 1024 * 1024

func FilterSamples(files []VideoFile) []VideoFile {
	// Group files by folder
	groups := make(map[string][]VideoFile)
	for _, f := range files {
		groups[f.FolderRelativePath] = append(groups[f.FolderRelativePath], f)
	}

	result := make([]VideoFile, 0, len(files))
	for _, group := range groups {
		maxSize := int64(0)
		for _, f := range group {
			if f.SizeBytes > maxSize {
				maxSize = f.SizeBytes
			}
		}
		for _, f := range group {
			if maxSize >= sampleThresholdBytes && f.SizeBytes < sampleThresholdBytes {
				continue
			}
			result = append(result, f)
		}
	}
	return result
}
