package slacklog

// EmojiTable : 絵文字データを保持する。
// urlMapは絵文字名をキーとし、画像が置いてあるURLが値である。
type EmojiTable struct {
	URLMap map[string]string
}

// NewEmojiTable : pathに指定したJSON形式の絵文字データを読み込み、EmojiTableを
// 生成する。
func NewEmojiTable(src LogSource, path string) (*EmojiTable, error) {
	emojis := &EmojiTable{
		URLMap: map[string]string{},
	}
	if err := ReadLogSourceAsJSON(src, path, &emojis.URLMap); err != nil {
		return nil, err
	}
	return emojis, nil
}
