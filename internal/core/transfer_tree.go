package core

import (
	"path"
	"sort"
	"strings"
)

// TransferNode is an observation of active transfers, not a manifest of
// all files still waiting. rclone does not expose its full file queue.
type TransferNode struct {
	Path       string          `json:"path"`
	Name       string          `json:"name"`
	Directory  bool            `json:"directory"`
	Bytes      int64           `json:"bytes"`
	TotalBytes int64           `json:"total_bytes"`
	Speed      float64         `json:"speed"`
	FileCount  int             `json:"file_count"`
	Children   []*TransferNode `json:"children,omitempty"`
}

func TransferTree(files []RCTransfer) []*TransferNode {
	root := &TransferNode{}
	directories := map[string]*TransferNode{"": root}
	for _, file := range files {
		name := strings.ReplaceAll(file.Name, `\`, "/")
		name = path.Clean("/" + name)
		name = strings.TrimPrefix(name, "/")
		if name == "" || name == "." {
			continue
		}
		parts := strings.Split(name, "/")
		parent := root
		for i, part := range parts {
			full := strings.Join(parts[:i+1], "/")
			if i == len(parts)-1 {
				parent.Children = append(parent.Children, &TransferNode{
					Path: full, Name: part, Bytes: file.Bytes, TotalBytes: file.Size, Speed: file.Speed, FileCount: 1,
				})
			} else {
				node, ok := directories[full]
				if !ok {
					node = &TransferNode{Path: full, Name: part, Directory: true}
					directories[full] = node
					parent.Children = append(parent.Children, node)
				}
				parent = node
			}
		}
	}
	var aggregate func(*TransferNode)
	aggregate = func(node *TransferNode) {
		if !node.Directory && node != root {
			return
		}
		sort.SliceStable(node.Children, func(i, j int) bool {
			a, b := node.Children[i], node.Children[j]
			if a.Directory != b.Directory {
				return a.Directory
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		})
		for _, child := range node.Children {
			aggregate(child)
			node.Bytes += child.Bytes
			node.TotalBytes += child.TotalBytes
			node.Speed += child.Speed
			node.FileCount += child.FileCount
		}
	}
	aggregate(root)
	return root.Children
}

func PendingJobs(jobs []Job) []Job {
	var pending []Job
	for _, job := range jobs {
		if job.Status == "pending" {
			pending = append(pending, job)
		}
	}
	sort.SliceStable(pending, func(i, j int) bool {
		if pending[i].Position != pending[j].Position {
			return pending[i].Position < pending[j].Position
		}
		if !pending[i].CreatedAt.Equal(pending[j].CreatedAt) {
			return pending[i].CreatedAt.Before(pending[j].CreatedAt)
		}
		return pending[i].ID < pending[j].ID
	})
	return pending
}
