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
	"os"
	"path/filepath"
	"sort"

	slacklog "github.com/vim-jp/slacklog/lib"
)

func ConvertExportedLogs(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: go run scripts/main.go convert_exported_logs {indir} {outdir}")
		return nil
	}

	inDir := filepath.Clean(args[0])
	outDir := filepath.Clean(args[1])

	channels, _, err := readChannels(filepath.Join(inDir, "channels.json"), []string{"*"})
	if err != nil {
		return fmt.Errorf("could not read channels.json: %w", err)
	}

	if err := os.MkdirAll(outDir, 0777); err != nil {
		return fmt.Errorf("could not create %s directory: %w", outDir, err)
	}

	err = copyFile(filepath.Join(inDir, "channels.json"), filepath.Join(outDir, "channels.json"))
	if err != nil {
		return err
	}

	err = copyFile(filepath.Join(inDir, "users.json"), filepath.Join(outDir, "users.json"))
	if err != nil {
		return err
	}

	for _, channel := range channels {
		messages, err := ReadAllMessages(filepath.Join(inDir, channel.Name))
		if err != nil {
			return err
		}
		for _, message := range messages {
			message.UserProfile = nil
			message.RemoveTokenFromURLs()
		}
		channelDir := filepath.Join(outDir, channel.ID)
		if err := os.MkdirAll(channelDir, 0777); err != nil {
			return fmt.Errorf("could not create %s directory: %w", channelDir, err)
		}
		messagesPerDay := groupMessagesByDay(messages)
		for key := range messagesPerDay {
			err = writeMessages(filepath.Join(channelDir, key+".json"), messagesPerDay[key])
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(from string, to string) error {
	r, err := ioutil.ReadFile(from)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(to, r, 0666)
}

func readChannels(channelsJsonPath string, cfgChannels []string) ([]slacklog.Channel, map[string]*slacklog.Channel, error) {
	var channels []slacklog.Channel
	err := slacklog.ReadFileAsJSON(channelsJsonPath, &channels)
	if err != nil {
		return nil, nil, err
	}
	channels = slacklog.FilterChannel(channels, cfgChannels)
	sort.SliceStable(channels, func(i, j int) bool {
		return channels[i].Name < channels[j].Name
	})
	channelMap := make(map[string]*slacklog.Channel, len(channels))
	for i := range channels {
		channelMap[channels[i].ID] = &channels[i]
	}
	return channels, channelMap, err
}

func ReadAllMessages(inDir string) ([]*slacklog.Message, error) {
	dir, err := os.Open(inDir)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var messages []*slacklog.Message
	for i := range names {
		var msgs []*slacklog.Message
		err := slacklog.ReadFileAsJSON(filepath.Join(inDir, names[i]), &msgs)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msgs...)
	}
	return messages, nil
}

func groupMessagesByDay(messages []*slacklog.Message) map[string][]*slacklog.Message {
	messagesPerDay := map[string][]*slacklog.Message{}
	for i := range messages {
		time := slacklog.TsToDateTime(messages[i].Ts).Format("2006-01-02")
		messagesPerDay[time] = append(messagesPerDay[time], messages[i])
	}
	return messagesPerDay
}

func writeMessages(filename string, messages []*slacklog.Message) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(messages)
	if err != nil {
		return err
	}
	return nil
}
