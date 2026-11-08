package core

// DataDirectory lets platform adapters put their own preferences beside the
// selected private application state, including isolated development profiles.
func (a *App) DataDirectory() string { return a.dataDir }
