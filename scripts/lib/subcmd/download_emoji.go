/*
リファクタリング中
処理をslacklog packageに移動していく。
一旦、必要な処理はすべてslacklog packageから一時的にエクスポートするか、このファ
イル内で定義している。
*/

package subcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/slack-go/slack"
)

func DownloadEmoji(args []string) error {
	slackToken := os.Getenv("SLACK_TOKEN")
	if slackToken == "" {
		return fmt.Errorf("$SLACK_TOKEN required")
	}

	if len(args) < 1 {
		fmt.Println("Usage: go run scripts/main.go download_emoji {emojis-dir}")
		return nil
	}

	emojisDir := filepath.Clean(args[0])
	emojiJSONPath := filepath.Join(emojisDir, "emoji.json")
	if 1 < len(args) {
		emojiJSONPath = filepath.Clean(args[1])
	}

	api := slack.New(slackToken)

	emojis, err := api.GetEmoji()
	if err != nil {
		return err
	}

	err = os.MkdirAll(emojisDir, 0777)
	if err != nil {
		return err
	}

	for name, url := range emojis {
		if url[:6] == "alias:" {
			continue
		}
		downloadEmojiToFile(url, name, emojisDir)
		emojis[name] = filepath.Ext(emojis[name])
	}

	// write `emojis` to a file as JSON, using with json.Encoder. this saves
	// memory to marshal JSON.
	f, err := os.Create(emojiJSONPath)
	if err != nil {
		return err
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(emojis)
	if err != nil {
		return err
	}

	return nil
}

func downloadEmojiToFile(url, name, basePath string) error {
	extension := filepath.Ext(url)
	destFile := filepath.Join(basePath, name+extension)
	if _, err := os.Stat(destFile); err == nil {
		return nil
	}

	fmt.Printf("Downloading: :%s:\n", name)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("[%s]: %s", resp.Status, url)
	}

	w, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = io.Copy(w, resp.Body)

	return err
}
