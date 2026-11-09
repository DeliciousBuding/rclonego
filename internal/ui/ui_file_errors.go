package desktop

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/egoist/mygo/ui"
)

type fileIssue struct {
	message, hint, action string
}

func explainFileError(err error) fileIssue {
	if err == nil {
		return fileIssue{"操作未完成", "查看详情，检查所选位置后重试。", ""}
	}
	var executable *exec.Error
	var network net.Error
	switch {
	case errors.As(err, &executable):
		return fileIssue{"无法启动 rclone 引擎", "在设置中安装或重新选择引擎。", "engine"}
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &network) && network.Timeout():
		return fileIssue{"读取超时", "检查网络连接后重试。", ""}
	case errors.Is(err, context.Canceled):
		return fileIssue{"读取已取消", "需要时可以重新读取。", ""}
	case errors.Is(err, os.ErrNotExist):
		return fileIssue{"所选位置不存在", "检查文件是否已移动或删除，再重新选择。", ""}
	case errors.Is(err, os.ErrPermission):
		return fileIssue{"无法访问所选位置", "检查此位置的读取权限后重试。", ""}
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "invalid_grant"), strings.Contains(message, "invalid_token"):
		return fileIssue{"账户授权需要检查", "在云端账户中重新授权后重试。", "account"}
	case message == "directory listing failed; check the path and account authorization":
		return fileIssue{"无法读取云端目录", "检查路径、网络和账户权限后重试。", "account"}
	case strings.Contains(message, "image preview is limited"), strings.Contains(message, "image preview supports"),
		strings.Contains(message, "this file has no text or image preview"):
		return fileIssue{"此文件无法在应用内预览", "下载后使用其他应用打开。", "download"}
	case strings.HasPrefix(message, "unsupported upload file:"):
		return fileIssue{"不支持此文件类型", "选择普通文件或文件夹后重新上传。", ""}
	case message == "enter one file or folder name without path separators":
		return fileIssue{"文件名称无效", "名称不能包含路径分隔符或控制字符。", ""}
	case message == "directory path exceeds the 32 kib limit", message == "remote directory must not contain parent traversal":
		return fileIssue{"目录路径无效", "选择完整的本机路径或云端目录。", ""}
	case strings.Contains(message, "use an absolute local directory"), strings.Contains(message, "local directory must be absolute"),
		strings.Contains(message, "directory path is required"):
		return fileIssue{"目录路径无效", "选择完整的本机路径或云端目录。", ""}
	default:
		return fileIssue{"操作未完成", "查看详情，检查所选位置后重试。", ""}
	}
}

func (s *nativeUI) fileErrorFeedback(c *ui.Context, err error, details *bool, retry, download func()) {
	issue, t := explainFileError(err), c.Theme()
	ui.Column(c).Shrink(0).Gap(6).Children(func() {
		ui.Text(c, text(c, issue.message)).FontSize(12).FontWeight(500).TextColor(t.Warning)
		ui.Text(c, text(c, issue.hint)).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if retry != nil && issue.action != "download" {
				ui.Button(c.Key("retry-file-read"), text(c, "重试")).Disabled(s.busy).OnClick(retry)
			}
			if issue.action == "download" && download != nil {
				ui.Button(c.Key("download-preview"), text(c, "下载…")).Disabled(s.busy).OnClick(download)
			}
			if issue.action == "account" {
				ui.Button(c.Key("check-file-account"), text(c, "检查账户")).OnClick(func() {
					s.filesUI.previewOpen = false
					s.page = "remotes"
				})
			}
			if issue.action == "engine" {
				ui.Button(c.Key("check-file-engine"), text(c, "管理引擎")).OnClick(func() {
					s.filesUI.previewOpen = false
					s.page, s.showEngineSettings = "settings", true
				})
			}
		})
		if err != nil {
			ui.Collapsible(c.Key("file-error-details"), text(c, "错误详情"), details, func() {
				ui.Text(c, err.Error()).FontSize(11).TextColor(t.TextMuted).MaxLines(4).Selectable().Tooltip(err.Error())
			})
		}
	})
}
