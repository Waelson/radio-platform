package prefs

// LineInStore provides Get/Set for the line-in default device ID, persisted
// within the shared Preferences file. Each call reads or writes the full file
// atomically so that concurrent preference changes from other subsystems are
// never silently discarded.
type LineInStore struct {
	path string
}

// NewLineInStore returns a LineInStore backed by the preferences file at path.
func NewLineInStore(path string) *LineInStore {
	return &LineInStore{path: path}
}

// GetLineInDefaultDeviceID returns the stored default device ID, or an empty
// string if no preference has been saved yet.
func (s *LineInStore) GetLineInDefaultDeviceID() string {
	return Load(s.path).LineInDefaultDeviceID
}

// SetLineInDefaultDeviceID saves the default device ID by loading the current
// preferences, updating the field, and writing the file atomically.
func (s *LineInStore) SetLineInDefaultDeviceID(id string) error {
	p := Load(s.path)
	p.LineInDefaultDeviceID = id
	return Save(s.path, p)
}
