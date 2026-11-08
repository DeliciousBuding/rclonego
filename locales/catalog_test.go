package locales

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestCatalogsHaveMatchingKeysAndPreserveFormats(t *testing.T) {
	formats := regexp.MustCompile(`%(?:\[\d+\])?[-+ #0]*(?:\d+|\*)?(?:\.(?:\d+|\*))?[a-zA-Z%]`)
	catalogs := make(map[string]map[string]json.RawMessage)
	for _, language := range []string{"en", "zh-Hans"} {
		data, err := files.ReadFile(language + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var entries map[string]json.RawMessage
		if err := json.Unmarshal(data, &entries); err != nil {
			t.Fatal(err)
		}
		catalogs[language] = entries
	}
	for key, english := range catalogs["en"] {
		if strings.HasPrefix(key, "@") {
			continue
		}
		chinese, exists := catalogs["zh-Hans"][key]
		if !exists {
			t.Fatalf("Chinese catalog lacks %s", key)
		}
		var en, zh string
		if json.Unmarshal(english, &en) != nil || json.Unmarshal(chinese, &zh) != nil {
			t.Fatalf("%s is not a string message", key)
		}
		if !reflect.DeepEqual(formats.FindAllString(en, -1), formats.FindAllString(zh, -1)) {
			t.Fatalf("%s changed a printf format: %q / %q", key, en, zh)
		}
	}
	if len(catalogs["en"]) != len(catalogs["zh-Hans"]) {
		t.Fatal("Catalog keys differ")
	}
}

func TestStaticMessagesAndUnknownData(t *testing.T) {
	if got := TextIn("en", "任务队列"); got != "Task queue" {
		t.Fatalf("English navigation: %q", got)
	}
	if got := TextIn("zh-Hans", "任务队列"); got != "任务队列" {
		t.Fatalf("Chinese navigation: %q", got)
	}
	if got := fmt.Sprintf(TextIn("en", "已传 %d / %d 个文件"), 2, 5); got != "Transferred 2 / 5 files" {
		t.Fatalf("Transfer progress: %q", got)
	}
	for _, value := range []string{
		"example-remote:用户文档/完成.txt",
		"/synthetic/任务队列/report.txt",
		"A user-supplied task name",
		"ERROR: 复制失败; engine output",
	} {
		if got := TextIn("en", value); got != value {
			t.Fatalf("Non-message data was translated: %q / %q", value, got)
		}
	}
}

func TestTypedCountsAndDates(t *testing.T) {
	if got := In("en").FilesCount(1); got != "1 file" {
		t.Fatalf("Singular file count: %q", got)
	}
	if got := In("en").FilesCount(2); got != "2 files" {
		t.Fatalf("Plural file count: %q", got)
	}
	if got := In("zh-Hans").FilesCount(2); got != "2 个文件" {
		t.Fatalf("Chinese file count: %q", got)
	}
	date := time.Date(2026, time.January, 5, 13, 7, 0, 0, time.UTC)
	if got := In("en").RunCreated(date); !strings.Contains(got, "2026") || strings.Contains(got, "{") {
		t.Fatalf("Creation date was not formatted: %q", got)
	}
}
