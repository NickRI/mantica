package main

import "golang.org/x/sys/unix"

type StorageInfo struct {
	Total     uint64 `json:"total"`
	Free      uint64 `json:"free"`
	Used      uint64 `json:"used"`
	MapsBytes int64  `json:"maps_bytes"`
}

func (a *App) storageInfo() (StorageInfo, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(a.dir, &st); err != nil {
		return StorageInfo{}, err
	}
	bsize := uint64(st.Bsize)
	total := st.Blocks * bsize
	free := st.Bavail * bsize
	used := total - free
	var mapsBytes int64
	maps, err := a.listMaps()
	if err != nil {
		return StorageInfo{}, err
	}
	for _, m := range maps {
		mapsBytes += m.Size
	}
	return StorageInfo{Total: total, Free: free, Used: used, MapsBytes: mapsBytes}, nil
}
