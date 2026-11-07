package core

import (
	"errors"
	"strings"
)

func (a *App) SaveMounts(configs []MountConfig) error {
	a.mountMu.Lock()
	defer a.mountMu.Unlock()
	ids, targets := map[string]bool{}, map[string]bool{}
	for i := range configs {
		configs[i].Drive = NormalizeMountTarget(configs[i].Drive)
		configs[i].Remote = EnsureColonRemote(strings.TrimSpace(configs[i].Remote))
		if err := ValidateMountConfig(configs[i]); err != nil {
			return err
		}
		if ids[configs[i].ID] || targets[configs[i].Drive] {
			return errors.New("mount ids and targets must be unique")
		}
		ids[configs[i].ID], targets[configs[i].Drive] = true, true
	}
	return a.store.Update(func(state *State) error {
		for _, old := range state.Mounts {
			if pid := state.OwnedMounts[old.ID]; pid > 0 && processExists(pid) {
				matches := false
				for _, cfg := range configs {
					if cfg == old {
						matches = true
						break
					}
				}
				if !matches {
					return errors.New("stop the owned mount before editing or removing its configuration")
				}
			}
		}
		state.Mounts = append([]MountConfig(nil), configs...)
		for id := range state.OwnedMounts {
			if !ids[id] {
				delete(state.OwnedMounts, id)
				delete(state.MountIdentities, id)
			}
		}
		return nil
	})
}
