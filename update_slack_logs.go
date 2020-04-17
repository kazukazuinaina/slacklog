package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
)

func main() {
	if err := doMain(); err != nil {
		fmt.Fprintf(os.Stderr, "[error] %s\n", err)
		os.Exit(1)
	}
}

func doMain() error {
	if len(os.Args) < 5 {
		fmt.Println("Usage: go run scripts/update_slack_logs.go {config.json} {templatedir} {indir} {outdir}")
		return nil
	}

	configJsonPath := filepath.Clean(os.Args[1])
	templateDir := filepath.Clean(os.Args[2])
	inDir := filepath.Clean(os.Args[3])
	outDir := filepath.Clean(os.Args[4])

	cfg, err := readConfig(configJsonPath)
	if err != nil {
		return fmt.Errorf("could not read config: %s", err)
	}
	_, userMap, err := readUsers(filepath.Join(inDir, "users.json"))
	if err != nil {
		return fmt.Errorf("could not read users.json: %s", err)
	}
	channels, _, err := readChannels(filepath.Join(inDir, "channels.json"), cfg.Channels)
	if err != nil {
		return fmt.Errorf("could not read channels.json: %s", err)
	}

	if err := mkdir(outDir); err != nil {
		return fmt.Errorf("could not create out directory: %s", err)
	}

	emptyChannel := make(map[string]bool, len(channels))
	for i := range channels {
		if err := mkdir(filepath.Join(outDir, channels[i].Name)); err != nil {
			return fmt.Errorf("could not create %s/%s directory: %s", outDir, channels[i].Name, err)
		}
		msgMap, threadMap, err := getMsgPerMonth(inDir, channels[i].Name)
		if len(msgMap) == 0 {
			emptyChannel[channels[i].Name] = true
			continue
		}
		if err != nil {
			return err
		}
		// Generate {outdir}/{channel}/index.html (links to {channel}/{year}/{month})
		content, err := genChannelIndex(inDir, filepath.Join(templateDir, "channel_index.tmpl"), &channels[i], msgMap, cfg)
		if err != nil {
			return fmt.Errorf("could not generate %s/%s: %s", outDir, channels[i].Name, err)
		}
		err = ioutil.WriteFile(filepath.Join(outDir, channels[i].Name, "index.html"), content, 0666)
		if err != nil {
			return fmt.Errorf("could not create %s/%s/index.html: %s", outDir, channels[i].Name, err)
		}
		// Generate {outdir}/{channel}/{year}/{month}/index.html
		for _, msgPerMonth := range msgMap {
			if err := mkdir(filepath.Join(outDir, channels[i].Name, msgPerMonth.Year, msgPerMonth.Month)); err != nil {
				return fmt.Errorf("could not create %s/%s/%s/%s directory: %s", outDir, channels[i].Name, msgPerMonth.Year, msgPerMonth.Month, err)
			}
			content, err := genChannelPerMonthIndex(inDir, filepath.Join(templateDir, "channel_per_month_index.tmpl"), &channels[i], msgPerMonth, userMap, threadMap, cfg)
			if err != nil {
				return fmt.Errorf("could not generate %s/%s/%s/%s/index.html: %s", outDir, channels[i].Name, msgPerMonth.Year, msgPerMonth.Month, err)
			}
			err = ioutil.WriteFile(filepath.Join(outDir, channels[i].Name, msgPerMonth.Year, msgPerMonth.Month, "index.html"), content, 0666)
			if err != nil {
				return fmt.Errorf("could not create %s/%s/index.html: %s", outDir, channels[i].Name, err)
			}
		}
	}

	// Remove empty channels
	newChannels := make([]channel, 0, len(channels))
	for i := range channels {
		if !emptyChannel[channels[i].Name] {
			newChannels = append(newChannels, channels[i])
		}
	}
	channels = newChannels

	// Generate {outdir}/index.html (links to {channel})
	content, err := genIndex(channels, filepath.Join(templateDir, "index.tmpl"), cfg)
	err = ioutil.WriteFile(filepath.Join(outDir, "index.html"), content, 0666)
	if err != nil {
		return fmt.Errorf("could not create %s/index.html: %s", outDir, err)
	}

	return nil
}

func mkdir(path string) error {
	os.MkdirAll(path, 0777)
	if fi, err := os.Stat(path); os.IsNotExist(err) || !fi.IsDir() {
		return err
	}
	return nil
}

func visibleMsg(msg *message) bool {
	return msg.Subtype == "" || msg.Subtype == "bot_message" || msg.Subtype == "thread_broadcast"
}

func genIndex(channels []channel, tmplFile string, cfg *config) ([]byte, error) {
	params := make(map[string]interface{})
	params["baseUrl"] = cfg.BaseUrl
	params["channels"] = channels
	var out bytes.Buffer
	name := filepath.Base(tmplFile)
	t, err := template.New(name).Delims("<<", ">>").ParseFiles(tmplFile)
	if err != nil {
		return nil, err
	}
	err = t.Execute(&out, params)
	return out.Bytes(), err
}

func genChannelIndex(inDir, tmplFile string, channel *channel, msgMap map[string]*msgPerMonth, cfg *config) ([]byte, error) {
	params := make(map[string]interface{})
	params["baseUrl"] = cfg.BaseUrl
	params["channel"] = channel
	params["msgMap"] = msgMap
	var out bytes.Buffer
	name := filepath.Base(tmplFile)
	t, err := template.New(name).Delims("<<", ">>").ParseFiles(tmplFile)
	if err != nil {
		return nil, err
	}
	err = t.Execute(&out, params)
	return out.Bytes(), err
}

func genChannelPerMonthIndex(inDir, tmplFile string, channel *channel, msgPerMonth *msgPerMonth, userMap map[string]*user, threadMap map[string][]*message, cfg *config) ([]byte, error) {
	params := make(map[string]interface{})
	params["baseUrl"] = cfg.BaseUrl
	params["channel"] = channel
	params["msgPerMonth"] = msgPerMonth
	params["threadMap"] = threadMap
	var out bytes.Buffer

	// TODO tokenize/parse message.Text
	var reLinkWithTitle = regexp.MustCompile(`&lt;(https?://[^>]+?\|(.+?))&gt;`)
	var reLink = regexp.MustCompile(`&lt;(https?://[^>]+?)&gt;`)
	// go regexp does not support back reference
	var reCode = regexp.MustCompile("`{3}|｀{3}")
	var reCodeShort = regexp.MustCompile("[`｀]([^`]+?)[`｀]")
	var reDel = regexp.MustCompile(`~([^~]+?)~`)
	var reMention = regexp.MustCompile(`&lt;@(\w+?)&gt;`)
	var reChannel = regexp.MustCompile(`&lt;#\w+?\|([^&]+?)&gt;`)
	var reNewline = regexp.MustCompile(`\n`)
	var escapeSpecialChars = func(text string) string {
		text = html.EscapeString(html.UnescapeString(text))
		text = strings.Replace(text, "{{", "&#123;&#123;", -1)
		return strings.Replace(text, "{%", "&#123;&#37;", -1)
	}
	var text2Html = func(text string) string {
		text = escapeSpecialChars(text)
		text = reNewline.ReplaceAllString(text, "<br>")
		chunks := reCode.Split(text, -1)
		for i := range chunks {
			if i%2 == 0 {
				chunks[i] = reLinkWithTitle.ReplaceAllString(chunks[i], "<a href='${1}'>${2}</a>")
				chunks[i] = reLink.ReplaceAllString(chunks[i], "<a href='${1}'>${1}</a>")
				chunks[i] = reCodeShort.ReplaceAllString(chunks[i], "<code>${1}</code>")
				chunks[i] = reDel.ReplaceAllString(chunks[i], "<del>${1}</del>")
				chunks[i] = reMention.ReplaceAllStringFunc(chunks[i], func(whole string) string {
					m := reMention.FindStringSubmatch(whole)
					if name := getDisplayNameByUserId(m[1], userMap); name != "" {
						return "@" + name
					}
					return whole
				})
				chunks[i] = reChannel.ReplaceAllStringFunc(chunks[i], func(whole string) string {
					channelName := reChannel.FindStringSubmatch(whole)[1]
					name := html.EscapeString(channelName)
					return "<a href='/slacklog/" + name + "/'>#" + name + "</a>"
				})
			} else {
				chunks[i] = "<pre>" + chunks[i] + "</pre>"
			}
		}
		return strings.Join(chunks, "")
	}
	var escapeText = func(text string) string {
		text = html.EscapeString(html.UnescapeString(text))
		text = reNewline.ReplaceAllString(text, " ")
		return text
	}
	var funcText = func(msg *message) string {
		text := text2Html(msg.Text)
		if msg.Edited != nil && cfg.EditedSuffix != "" {
			text += "<span class='slacklog-text-edited'>" + html.EscapeString(cfg.EditedSuffix) + "</span>"
		}
		return text
	}
	var funcAttachmentText = func(attachment *messageAttachment) string {
		return text2Html(attachment.Text)
	}
	var ts2datetime = func(ts string) time.Time {
		t := strings.Split(ts, ".")
		if len(t) != 2 {
			return time.Time{}
		}
		sec, err := strconv.ParseInt(t[0], 10, 64)
		if err != nil {
			return time.Time{}
		}
		nsec, err := strconv.ParseInt(t[0], 10, 64)
		if err != nil {
			return time.Time{}
		}
		japan, err := time.LoadLocation("Asia/Tokyo")
		if err != nil {
			return time.Time{}
		}
		return time.Unix(sec, nsec).In(japan)
	}
	var ts2threadMtime = func(ts string) time.Time {
		lastMsg := threadMap[ts][len(threadMap[ts])-1]
		return ts2datetime(lastMsg.Ts)
	}

	// TODO check below subtypes work correctly
	// TODO support more subtypes
	name := filepath.Base(tmplFile)
	t, err := template.New(name).
		Delims("<<", ">>").
		Funcs(map[string]interface{}{
			"visible": visibleMsg,
			"datetime": func(ts string) string {
				return ts2datetime(ts).Format("2日 15:04:05")
			},
			"username": func(msg *message) string {
				if msg.Subtype == "bot_message" {
					return escapeSpecialChars(msg.Username)
				}
				return escapeSpecialChars(getDisplayNameByUserId(msg.User, userMap))
			},
			"userIconUrl": func(msg *message) string {
				switch msg.Subtype {
				case "", "thread_broadcast":
					user, ok := userMap[msg.User]
					if !ok {
						return "" // TODO show default icon
					}
					return user.Profile.Image48
				case "bot_message":
					if msg.Icons != nil && msg.Icons.Image48 != "" {
						return msg.Icons.Image48
					}
				}
				return ""
			},
			"text":           funcText,
			"attachmentText": funcAttachmentText,
			"threadMtime": func(ts string) string {
				return ts2threadMtime(ts).Format("2日 15:04:05")
			},
			"threads": func(ts string) []*message {
				if threads, ok := threadMap[ts]; ok {
					return threads[1:]
				}
				return nil
			},
			"threadNum": func(ts string) int {
				return len(threadMap[ts]) - 1
			},
			"threadRootText": func(ts string) string {
				threads, ok := threadMap[ts]
				if !ok {
					return ""
				}
				runes := []rune(threads[0].Text)
				text := string(runes)
				if len(runes) > 20 {
					text = string(runes[:20]) + " ..."
				}
				return escapeText(text)
			},
		}).
		ParseFiles(tmplFile)
	if err != nil {
		return nil, err
	}
	err = t.Execute(&out, params)
	return out.Bytes(), err
}

func getDisplayNameByUserId(userId string, userMap map[string]*user) string {
	if user, ok := userMap[userId]; ok {
		if user.Profile.RealName != "" {
			return user.Profile.RealName
		}
		if user.Profile.DisplayName != "" {
			return user.Profile.DisplayName
		}
	}
	return ""
}

type msgPerMonth struct {
	Year     string
	Month    string
	Messages []message
}

// "{year}-{month}-{day}.json"
var reMsgFilename = regexp.MustCompile(`^(\d{4})-(\d{2})-\d{2}\.json$`)

func getMsgPerMonth(inDir string, channelName string) (map[string]*msgPerMonth, map[string][]*message, error) {
	dir, err := os.Open(filepath.Join(inDir, channelName))
	if err != nil {
		return nil, nil, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(0)
	sort.SliceStable(names, func(i, j int) bool {
		return names[i] < names[j]
	})
	if err != nil {
		return nil, nil, err
	}
	msgMap := make(map[string]*msgPerMonth)
	threadMap := make(map[string][]*message)
	for i := range names {
		m := reMsgFilename.FindStringSubmatch(names[i])
		if len(m) == 0 {
			fmt.Fprintf(os.Stderr, "[warning] skipping %s/%s/%s ...", inDir, channelName, names[i])
			continue
		}
		key := m[1] + m[2]
		if _, ok := msgMap[key]; !ok {
			msgMap[key] = &msgPerMonth{Year: m[1], Month: m[2]}
		}
		err := readMessages(filepath.Join(inDir, channelName, names[i]), msgMap[key], threadMap)
		if err != nil {
			return nil, nil, err
		}
	}
	for key := range msgMap {
		if len(msgMap[key].Messages) == 0 {
			delete(msgMap, key)
			continue
		}
		sort.SliceStable(msgMap[key].Messages, func(i, j int) bool {
			// must be the same digits, so no need to convert the timestamp to a number
			return msgMap[key].Messages[i].Ts < msgMap[key].Messages[j].Ts
		})
	}
	return msgMap, threadMap, nil
}

type message struct {
	ClientMsgId string              `json:"client_msg_id"`
	Typ         string              `json:"type"`
	Subtype     string              `json:"subtype"`
	Text        string              `json:"text"`
	User        string              `json:"user"`
	Ts          string              `json:"ts"`
	ThreadTs    string              `json:"thread_ts"`
	Username    string              `json:"username"`
	BotId       string              `json:"bot_id"`
	Team        string              `json:"team"`
	UserTeam    string              `json:"user_team"`
	SourceTeam  string              `json:"source_team"`
	UserProfile messageUserProfile  `json:"user_profile"`
	Attachments []messageAttachment `json:"attachments"`
	// Blocks      []messageBlock     `json:"blocks"`    // TODO
	Reactions []messageReaction `json:"reactions"`
	Edited    *messageEdited    `json:"edited"`
	Icons     *messageIcons     `json:"icons"`
	Files     []messageFile     `json:"files"`
	Root      *message          `json:"root"`
}

type messageFile struct {
	Id                 string `json:"id"`
	Created            int64  `json:"created"`
	Timestamp          int64  `json:"timestamp"`
	Name               string `json:"name"`
	Title              string `json:"title"`
	Mimetype           string `json:"mimetype"`
	Filetype           string `json:"filetype"`
	PrettyType         string `json:"pretty_type"`
	User               string `json:"user"`
	Editable           bool   `json:"editable"`
	Size               int64  `json:"size"`
	Mode               string `json:"mode"`
	IsExternal         bool   `json:"is_external"`
	ExternalType       string `json:"external_type"`
	IsPublic           bool   `json:"is_public"`
	PublicUrlShared    bool   `json:"public_url_shared"`
	DisplayAsBot       bool   `json:"display_as_bot"`
	Username           string `json:"username"`
	UrlPrivate         string `json:"url_private"`
	UrlPrivateDownload string `json:"url_private_download"`
	Permalink          string `json:"permalink"`
	PermalinkPublic    string `json:"permalink_public"`
	EditLink           string `json:"edit_link"`
	IsStarred          bool   `json:"is_starred"`
	HasRichPreview     bool   `json:"has_rich_preview"`
}

type messageIcons struct {
	Image48 string `json:"image_48"`
}

type messageEdited struct {
	User string `json:"user"`
	Ts   string `json:"ts"`
}

type messageUserProfile struct {
	AvatarHash        string `json:"avatar_hash"`
	Image72           string `json:"image72"`
	FirstName         string `json:"first_name"`
	RealName          string `json:"real_name"`
	DisplayName       string `json:"display_name"`
	Team              string `json:"team"`
	Name              string `json:"name"`
	IsRestricted      bool   `json:"is_restricted"`
	IsUltraRestricted bool   `json:"is_ultra_restricted"`
}

type messageBlock struct {
	Typ      string                `json:"type"`
	Elements []messageBlockElement `json:"elements"`
}

type messageBlockElement struct {
	Typ       string `json:"type"`
	Name      string `json:"name"`       // for type = "emoji"
	Text      string `json:"text"`       // for type = "text"
	ChannelId string `json:"channel_id"` // for type = "channel"
}

type messageAttachment struct {
	ServiceName     string `json:"service_name"`
	AuthorIcon      string `json:"author_icon"`
	AuthorName      string `json:"author_name"`
	AuthorSubname   string `json:"author_subname"`
	Title           string `json:"title"`
	TitleLink       string `json:"title_link"`
	Text            string `json:"text"`
	Fallback        string `json:"fallback"`
	ThumbUrl        string `json:"thumb_url"`
	FromUrl         string `json:"from_url"`
	ThumbWidth      int    `json:"thumb_width"`
	ThumbHeight     int    `json:"thumb_height"`
	ServiceIcon     string `json:"service_icon"`
	Id              int    `json:"id"`
	OriginalUrl     string `json:"original_url"`
	VideoHtml       string `json:"video_html"`
	VideoHtmlWidth  int    `json:"video_html_width"`
	VideoHtmlHeight int    `json:"video_html_height"`
	Footer          string `json:"footer"`
	FooterIcon      string `json:"footer_icon"`
}

type messageReaction struct {
	Name  string   `json:"name"`
	Users []string `json:"users"`
	Count int      `json:"count"`
}

func readMessages(msgJsonPath string, msgPerMonth *msgPerMonth, threadMap map[string][]*message) error {
	content, err := ioutil.ReadFile(msgJsonPath)
	if err != nil {
		return err
	}
	var msgs []message
	err = json.Unmarshal(content, &msgs)
	if err != nil {
		return fmt.Errorf("failed to unmarshal %s: %s", msgJsonPath, err)
	}
	for i := range msgs {
		if !visibleMsg(&msgs[i]) {
			continue
		}
		rootMsgOfThread := msgs[i].ThreadTs == msgs[i].Ts
		if msgs[i].ThreadTs == "" || rootMsgOfThread ||
			msgs[i].Subtype == "thread_broadcast" ||
			msgs[i].Subtype == "bot_message" {
			msgPerMonth.Messages = append(msgPerMonth.Messages, msgs[i])
		}
		if msgs[i].ThreadTs != "" {
			if rootMsgOfThread {
				threadTs := msgs[i].ThreadTs
				replies := threadMap[msgs[i].ThreadTs]
				for j := 0; j < len(replies); {    // remove root message(s)
					if replies[j].Ts == threadTs {
						replies = append(replies[:j], replies[j+1:]...)
						continue
					}
					j++
				}
				threadMap[msgs[i].ThreadTs] = append([]*message{&msgs[i]}, replies...)
			} else {
				threadMap[msgs[i].ThreadTs] = append(threadMap[msgs[i].ThreadTs], &msgs[i])
			}
		}
	}
	return nil
}

type config struct {
	EditedSuffix string   `json:"edited_suffix"`
	BaseUrl      string   `json:"base_url"`
	Channels     []string `json:"channels"`
}

func readConfig(configPath string) (*config, error) {
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg config
	err = json.Unmarshal(content, &cfg)
	return &cfg, err
}

type user struct {
	Id                string      `json:"id"`
	TeamId            string      `json:"team_id"`
	Name              string      `json:"name"`
	Deleted           bool        `json:"deleted"`
	Color             string      `json:"color"`
	RealName          string      `json:"real_name"`
	Tz                string      `json:"tz"`
	TzLabel           string      `json:"tz_label"`
	TzOffset          int         `json:"tz_offset"` // tzOffset / 60 / 60 = [-+] hour
	Profile           userProfile `json:"profile"`
	IsAdmin           bool        `json:"is_admin"`
	IsOwner           bool        `json:"is_owner"`
	IsPrimaryOwner    bool        `json:"is_primary_owner"`
	IsRestricted      bool        `json:"is_restricted"`
	IsUltraRestricted bool        `json:"is_ultra_restricted"`
	IsBot             bool        `json:"is_bot"`
	IsAppUser         bool        `json:"is_app_user"`
	Updated           int64       `json:"updated"`
}

type userProfile struct {
	Title                 string      `json:"title"`
	Phone                 string      `json:"phone"`
	Skype                 string      `json:"skype"`
	RealName              string      `json:"real_name"`
	RealNameNormalized    string      `json:"real_name_normalized"`
	DisplayName           string      `json:"display_name"`
	DisplayNameNormalized string      `json:"display_name_normalized"`
	Fields                interface{} `json:"fields"` // TODO ???
	StatusText            string      `json:"status_text"`
	StatusEmoji           string      `json:"status_emoji"`
	StatusExpiration      int64       `json:"status_expiration"`
	AvatarHash            string      `json:"avatar_hash"`
	FirstName             string      `json:"first_name"`
	LastName              string      `json:"last_name"`
	Image24               string      `json:"image_24"`
	Image32               string      `json:"image_32"`
	Image48               string      `json:"image_48"`
	Image72               string      `json:"image_72"`
	Image192              string      `json:"image_192"`
	Image512              string      `json:"image_512"`
	StatusTextCanonical   string      `json:"status_text_canonical"`
	Team                  string      `json:"team"`
	BotId                 string      `json:"bot_id"`
}

func readUsers(usersJsonPath string) ([]user, map[string]*user, error) {
	content, err := ioutil.ReadFile(usersJsonPath)
	if err != nil {
		return nil, nil, err
	}
	var users []user
	err = json.Unmarshal(content, &users)
	userMap := make(map[string]*user, len(users))
	for i := range users {
		userMap[users[i].Id] = &users[i]
		if users[i].Profile.BotId != "" {
			userMap[users[i].Profile.BotId] = &users[i]
		}
	}
	return users, userMap, err
}

type channel struct {
	Id         string         `json:"id"`
	Name       string         `json:"name"`
	Created    int64          `json:"created"`
	Creator    string         `json:"creator"`
	IsArchived bool           `json:"is_archived"`
	IsGeneral  bool           `json:"is_general"`
	Members    []string       `json:"members"`
	Pins       []channelPin   `json:"pins"`
	Topic      channelTopic   `json:"topic"`
	Purpose    channelPurpose `json:"purpose"`
}

type channelPin struct {
	Id      string `json:"id"`
	Typ     string `json:"type"`
	Created int64  `json:"created"`
	User    string `json:"user"`
	Owner   string `json:"owner"`
}

type channelTopic struct {
	Value   string `json:"value"`
	Creator string `json:"creator"`
	LastSet int64  `json:"last_set"`
}

type channelPurpose struct {
	Value   string `json:"value"`
	Creator string `json:"creator"`
	LastSet int64  `json:"last_set"`
}

func readChannels(channelsJsonPath string, cfgChannels []string) ([]channel, map[string]*channel, error) {
	content, err := ioutil.ReadFile(channelsJsonPath)
	if err != nil {
		return nil, nil, err
	}
	var channels []channel
	err = json.Unmarshal(content, &channels)
	channels = filterChannel(channels, cfgChannels)
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Name < channels[j].Name
	})
	channelMap := make(map[string]*channel, len(channels))
	for i := range channels {
		channelMap[channels[i].Id] = &channels[i]
	}
	return channels, channelMap, err
}

func filterChannel(channels []channel, cfgChannels []string) []channel {
	newChannels := make([]channel, 0, len(channels))
	for i := range cfgChannels {
		if cfgChannels[i] == "*" {
			return channels
		}
		for j := range channels {
			if cfgChannels[i] == channels[j].Name {
				newChannels = append(newChannels, channels[j])
				break
			}
		}
	}
	return newChannels
}
