package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DeliciousBuding/rclonego/locales"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type workflowUIState struct {
	editorOpen       bool
	editorID         string
	name             string
	source           string
	destination      string
	entries          string
	verification     string
	transfers        string
	checkers         string
	chunk            string
	buffer           string
	bandwidth        string
	cpu              string
	advanced         bool
	definitionSearch string
	definitionList   ui.ListState
	historySearch    string
	historyStatus    string
	historyRows      []core.Run
	historyOffset    int
	historyLoaded    bool
	historyHasNext   bool
	report           *core.RunReport
	reportList       ui.ListState
}

func (s *nativeUI) beginWorkflowEditor(definition core.WorkflowDefinition, clone bool) {
	w := &s.workflowUI
	if definition.Settings.Transfers == 0 {
		definition.Settings = s.defaults
	}
	w.editorID, w.name = definition.ID, definition.Name
	if clone {
		w.editorID, w.name = "", definition.Name+locales.CopySuffix()
	}
	w.source, w.destination = definition.Source, definition.Destination
	w.entries = strings.Join(definition.Entries, "\n")
	w.verification = map[string]string{"": "后端校验", "backend": "后端校验", "content": "内容比较", "size": "大小比较"}[definition.VerificationMode]
	w.transfers, w.checkers, w.cpu = itoa(definition.Settings.Transfers), itoa(definition.Settings.Checkers), itoa(definition.Settings.CPUParallelism)
	w.chunk, w.buffer, w.bandwidth = definition.Settings.ChunkSize, definition.Settings.BufferSize, definition.Settings.Bandwidth
	w.advanced, w.editorOpen = false, true
	s.isError, s.notice = false, ""
}

func editedWorkflow(w workflowUIState) (core.WorkflowDefinition, error) {
	definition := core.WorkflowDefinition{
		ID: w.editorID, Name: strings.TrimSpace(w.name), Source: strings.TrimSpace(w.source),
		Destination: strings.TrimSpace(w.destination),
	}
	if definition.Name == "" || definition.Source == "" || definition.Destination == "" {
		return definition, errors.New(locales.Text("请填写工作流名称、源目录和目标目录"))
	}
	seen := make(map[string]bool)
	for _, name := range strings.Split(w.entries, "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if filepath.Base(name) != name || strings.ContainsAny(name, "/\\") || name == "." || name == ".." || seen[name] {
			return definition, errors.New(locales.Text("步骤必须是互不重复的顶层目录或文件名，每行一项"))
		}
		seen[name] = true
		definition.Entries = append(definition.Entries, name)
	}
	definition.VerificationMode = map[string]string{"后端校验": "backend", "内容比较": "content", "大小比较": "size"}[w.verification]
	if definition.VerificationMode == "" {
		return definition, errors.New(locales.Text("请选择最终校验方式"))
	}
	var err error
	if definition.Settings.Transfers, err = strconv.Atoi(strings.TrimSpace(w.transfers)); err != nil {
		return definition, errors.New(locales.Text("传输并发必须是整数"))
	}
	if definition.Settings.Checkers, err = strconv.Atoi(strings.TrimSpace(w.checkers)); err != nil {
		return definition, errors.New(locales.Text("校验并发必须是整数"))
	}
	if definition.Settings.CPUParallelism, err = strconv.Atoi(strings.TrimSpace(w.cpu)); err != nil {
		return definition, errors.New(locales.Text("CPU 并行度必须是整数"))
	}
	definition.Settings.ChunkSize, definition.Settings.BufferSize = strings.TrimSpace(w.chunk), strings.TrimSpace(w.buffer)
	definition.Settings.Bandwidth = strings.TrimSpace(w.bandwidth)
	return definition, definition.Settings.Validate()
}

func (s *nativeUI) saveWorkflowEditor() {
	definition, err := editedWorkflow(s.workflowUI)
	if err != nil {
		s.setError(err)
		return
	}
	s.runAfter(locales.Text("保存工作流"), func() error {
		_, err := s.app.SaveWorkflow(definition)
		return err
	}, func() { s.workflowUI.editorOpen = false })
}

func (s *nativeUI) workflowOverlays(c *ui.Context) {
	w := &s.workflowUI
	title := "新建工作流"
	if w.editorID != "" {
		title = "编辑工作流"
	}
	// MyGo DialogBase provides modal focus, dismissal and accessibility.
	ui.DialogBase(c.Key("workflow-editor"), &w.editorOpen, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(ui.RGBA(25, 30, 40, .16))
		panel.Width(540).MaxWidthPercent(90).MaxHeightPercent(88).Padding(22).Gap(16).
			Radius(16).Background(t.Background).Shadow(0, 12, 36, 0, ui.RGBA(0, 0, 0, .12))
		ui.Row(c).Children(func() {
			ui.Text(c, text(c, title)).FontSize(17).FontWeight(600).Grow(1).MinWidth(0)
			iconButton(c.Key("close-workflow-editor"), iconClose, "关闭工作流编辑").OnClick(func() { w.editorOpen = false })
		})
		contentScroll(c).Gap(14).Children(func() {
			ui.Form(c, func() {
				ui.Field(c, text(c, "工作流名称"), func() { ui.TextInput(c.Key("workflow-name"), &w.name).AutoFocus().Placeholder(text(c, "文档归档")) })
				ui.Field(c, text(c, "源目录"), func() {
					s.pathInput(c.Key("workflow-source"), &w.source, "工作流源目录", "本机目录", true, false, nil)
				})
				ui.Field(c, text(c, "目标目录"), func() {
					s.storagePathInput(c.Key("workflow-destination"), &w.destination, "工作流目标目录", false)
				})
				ui.Field(c, text(c, "步骤顺序"), func() {
					ui.TextArea(c.Key("workflow-entries"), &w.entries).Height(84).
						Label(text(c, "工作流步骤顺序")).Placeholder(text(c, "每行一个顶层目录或文件名；留空按名称排序"))
				})
				ui.Field(c, text(c, "最终校验"), func() {
					translatedSelect(c.Key("workflow-verification"), &w.verification, []string{"后端校验", "内容比较", "大小比较"}).Label(text(c, "工作流最终校验"))
				})
			})
			ui.Text(c, text(c, "保存不会立即传输。运行时按步骤顺序复制，最后执行校验。")).FontSize(11).TextColor(t.TextMuted)
			if w.verification == "内容比较" {
				ui.Text(c, text(c, "内容比较会读取源和目标的完整文件，可能产生下载流量。")).FontSize(11).TextColor(t.Warning)
			}
			ui.Collapsible(c.Key("workflow-options"), text(c, "传输参数"), &w.advanced, func() {
				ui.Form(c, func() {
					ui.Field(c, text(c, "传输并发"), func() { ui.TextInput(c.Key("workflow-transfers"), &w.transfers) })
					ui.Field(c, text(c, "校验并发"), func() { ui.TextInput(c.Key("workflow-checkers"), &w.checkers) })
					ui.Field(c, text(c, "限速"), func() {
						ui.TextInput(c.Key("workflow-bandwidth"), &w.bandwidth).Placeholder(text(c, "留空表示不限速"))
					})
					if core.DestinationCapabilities(w.destination, s.remotes).DriveChunkSize {
						ui.Field(c, text(c, "Google Drive 上传分块"), func() { ui.TextInput(c.Key("workflow-chunk"), &w.chunk) })
					}
					ui.Field(c, text(c, "每路缓冲"), func() { ui.TextInput(c.Key("workflow-buffer"), &w.buffer) })
					ui.Field(c, text(c, "CPU 并行度"), func() { ui.TextInput(c.Key("workflow-cpu"), &w.cpu).Placeholder(text(c, "0 表示自动")) })
				})
			})
			if s.isError && s.notice != "" {
				ui.Text(c, s.notice).FontSize(12).TextColor(t.Danger).MaxLines(3)
			}
		})
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c.Key("cancel-workflow-editor"), text(c, "取消")).Height(30).OnClick(func() { w.editorOpen = false })
			ui.PrimaryButton(c.Key("save-workflow-editor"), text(c, "保存工作流")).Height(30).Disabled(s.busy).OnClick(s.saveWorkflowEditor)
		})
	})
	s.workflowReport(c)
}

const maxWorkflowImportBytes = 1 << 20

// Imports create a new local definition and never start a run.
func readPortableWorkflow(path string) (core.WorkflowDefinition, error) {
	var definition core.WorkflowDefinition
	file, err := os.Open(path)
	if err != nil {
		return definition, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxWorkflowImportBytes+1))
	if err != nil {
		return definition, err
	}
	if len(raw) > maxWorkflowImportBytes {
		return definition, errors.New(locales.Text("工作流文件超过 1 MiB"))
	}
	if err = json.Unmarshal(raw, &definition); err != nil {
		return definition, fmt.Errorf(locales.Text("工作流 JSON 无效：%w"), err)
	}
	definition.ID, definition.CreatedAt = "", time.Time{}
	return definition, nil
}

func writePortableWorkflow(path string, definition core.WorkflowDefinition) (err error) {
	definition.ID, definition.CreatedAt = "", time.Time{}
	raw, err := json.MarshalIndent(definition, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".workflow-export-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(temporary)
	}()
	if err = file.Chmod(0o600); err != nil {
		return err
	}
	if _, err = file.Write(append(raw, '\n')); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func (s *nativeUI) importWorkflow() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: s.win, Title: "导入工作流", Filters: []mygo.FileFilter{{Name: "工作流 JSON", Extensions: []string{"json"}}},
		})
		if err == nil && len(paths) == 0 {
			return
		}
		var definition core.WorkflowDefinition
		if err == nil {
			definition, err = readPortableWorkflow(paths[0])
		}
		s.win.Update(func() {
			if err != nil {
				s.setError(err)
				return
			}
			s.beginWorkflowEditor(definition, false)
		})
	}()
}

func (s *nativeUI) exportWorkflow(definition core.WorkflowDefinition) {
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent: s.win, Title: "导出工作流", DefaultPath: "workflow.json",
			Filters: []mygo.FileFilter{{Name: "工作流 JSON", Extensions: []string{"json"}}},
		})
		if err == nil && path == "" {
			return
		}
		if err == nil {
			err = writePortableWorkflow(path, definition)
		}
		s.win.Update(func() {
			if err != nil {
				s.setError(err)
			} else {
				s.isError, s.notice = false, "工作流已导出"
				s.noticeUntil = time.Now().Add(4 * time.Second)
			}
		})
	}()
}
