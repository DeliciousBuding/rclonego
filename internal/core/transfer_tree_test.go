package core

import (
	"testing"
	"time"
)

func TestTransferTreeAggregatesNestedFoldersAndStableNames(t *testing.T) {
	tree := TransferTree([]RCTransfer{
		{Name: "day/sub/b.jpg", Bytes: 3, Size: 10, Speed: 20},
		{Name: `day\sub\a.jpg`, Bytes: 5, Size: 20, Speed: 30},
		{Name: "root.txt", Bytes: 1, Size: 2},
	})
	if len(tree) != 2 || !tree[0].Directory || tree[0].FileCount != 2 || tree[0].Bytes != 8 || tree[0].TotalBytes != 30 {
		t.Fatalf("wrong hierarchy: %+v", tree)
	}
	children := tree[0].Children[0].Children
	if children[0].Name != "a.jpg" || children[1].Name != "b.jpg" {
		t.Fatal("files are not sorted")
	}
}

func TestQueueReorderPersistsAndScheduleBlocksFirstTask(t *testing.T) {
	app := testApp(t)
	one, err := app.queue.Add(JobCreateRequest{Name: "first", Operation: "copy", Source: "s", Destination: "d"})
	if err != nil {
		t.Fatal(err)
	}
	two, err := app.queue.Add(JobCreateRequest{Name: "second", Operation: "copy", Source: "s", Destination: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if err = app.queue.Move(two.ID, -1); err != nil {
		t.Fatal(err)
	}
	next, ok := nextPending(app.store.Snapshot().Jobs)
	if !ok || next.ID != two.ID {
		t.Fatalf("queue move failed: %+v", next)
	}
	future := time.Now().Add(time.Hour)
	if err = app.store.Update(func(state *State) error {
		for i := range state.Jobs {
			if state.Jobs[i].ID == two.ID {
				state.Jobs[i].NotBefore = &future
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok = nextPending(app.store.Snapshot().Jobs); ok {
		t.Fatal("scheduled first task was bypassed")
	}
	if err = app.queue.Move(one.ID, -1); err != nil {
		t.Fatal(err)
	}
	next, ok = nextPending(app.store.Snapshot().Jobs)
	if !ok || next.ID != one.ID {
		t.Fatal("ready task not eligible after explicit reorder")
	}
}
