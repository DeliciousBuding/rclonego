//go:build windows

package core

func syntheticMountTarget(_ string) string {
	for _, drive := range []string{"W:", "V:", "U:", "T:", "S:", "R:", "Q:", "P:"} {
		if !pathExists(drive + `\`) {
			return drive
		}
	}
	return ""
}
