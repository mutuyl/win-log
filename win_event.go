package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// EventCache WinEvent 缓存。
var EventCache map[int64]*WinEvent

// EventMessage WinEvent 消息部分
type EventMessage struct {
	Description string
	Details     string
}

// WinEvent 数据结构体。
type WinEvent struct {
	Id                   int64
	Version              int64
	Qualifiers           string
	Level                int64
	Task                 int64
	Opcode               int64
	Keywords             int64
	RecordId             int64
	ProviderName         string
	ProviderId           string
	LogName              string
	ProcessId            int64
	ThreadId             int64
	MachineName          string
	UserId               int64
	TimeCreated          string
	ActivityId           string
	RelatedActivityId    int64
	ContainerLog         string
	MatchedQueryIds      string
	Bookmark             string
	LevelDisplayName     string
	OpcodeDisplayName    string
	TaskDisplayName      string
	KeywordsDisplayNames string
	Properties           string
	Message              EventMessage
}

func (we WinEvent) String() string {
	return fmt.Sprintf("%d\t%s\t%d\t%s\t\t%s\n", we.Id, we.TimeCreated, we.RecordId, we.TaskDisplayName, we.Message.Description)
}

// WinEvent 列表排序。
type WinEvents []WinEvent

func (wes WinEvents) Len() int {
	return len(wes)
}
func (wes WinEvents) Less(i, j int) bool {
	return wes[i].RecordId < wes[j].RecordId
}
func (wes WinEvents) Swap(i, j int) {
	wes[i], wes[j] = wes[j], wes[i]
}

// GetWinEvent PowerShell 5.1 以上的版本，抓取日志信息。
func GetWinEvent(beginTime, endTime string) {
	var out bytes.Buffer
	cmd := exec.Command("powershell")

	cmd.Stdin = strings.NewReader(fmt.Sprintf(CmdVars, beginTime, endTime) + CmdWinEvent + CmdExit)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("Command 'Get-WinEvent' run: [%w]", err))
	}
	c, e := simplifiedchinese.GBK.NewDecoder().Bytes(out.Bytes())
	if e != nil {
		panic(fmt.Errorf("Decoding output: [%w]", e))
	}

	events, err := parseWinEvent(string(c), CmdWinEvent+"\r\n\r\n", "\r\n\r\n\r\n")
	if err != nil {
		panic(err)
	}
	wes := WinEvents(events)
	sort.Sort(wes)

	l := wes.Len()

	// 清空缓存。
	EventCache = make(map[int64]*WinEvent)

	if l > 0 {
		for i := 0; i < l; i++ {
			we := wes[i]
			EventCache[we.RecordId] = &we

			// 传输到日志服务器。
			// 根据配置的 EventId 过滤。
			if cfg.WinEvtIds == "" {
				buf, err := json.Marshal(we)
				if err != nil {
					panic(err)
				}
				fmt.Println()
				logger.Info("%s", string(buf))
			} else {
				if strings.Contains(cfg.WinEvtIds, strconv.FormatInt(we.Id, 10)) {
					buf, err := json.Marshal(we)
					if err != nil {
						panic(err)
					}
					fmt.Println()
					logger.Info("%s", string(buf))
				}
			}
		}
	}
}

// parseWinEvent 截取日志中 WinEvent 数据部分。
func parseWinEvent(data string, bes string, bee string) (events []WinEvent, err error) {
	if data != "" {
		bsi := strings.Index(data, bes)
		bei := strings.Index(data, bee)
		if bsi >= 0 && bei >= 0 && bei >= bsi {
			data = data[bsi+len(bes) : bei]
		}

		if data == "" {
			return
		}

		sepmsg := "Message"
		sepid := "Id"
		items := strings.Split(data, "\r\n"+sepmsg)
		l := len(items)
		if l > 0 {
			events = make([]WinEvent, 0)
			for _, v := range items {
				we := WinEvent{}

				// 将 Message 间隔字符串还原。
				// 生成完整的 WinEvent 数据。
				v = sepmsg + v
				// 获取 Id 属性的索引。
				idi := strings.Index(v, sepid)
				if idi >= 0 {
					// Message 部分日志。
					msg := v[:idi]
					message := parseEventMessage(msg)
					we.Message = *message

					pmap := make(map[string]string)
					// 键值对属性部分日志。
					content := v[idi:]
					// 按行分解。
					kvs := strings.Split(content, "\r\n")
					for _, kv := range kvs {
						if kv != "" && strings.Contains(kv, ":") {
							bi := strings.Index(kv, ":")
							pmap[strings.TrimSpace(kv[:bi])] = strings.TrimSpace(kv[bi+1:])
						}
					}

					err = parseEventMap(pmap, &we)
					if err != nil {
						return nil, err
					}

					// 和上次缓存的 WinEvent 对比过滤。
					var ec *WinEvent
					if EventCache != nil && len(EventCache) > 0 {
						ec = EventCache[we.RecordId]
					}
					if ec == nil {
						events = append(events, we)
					}
				}
			}
		}
	}
	return
}

func parseEventMessage(msg string) (message *EventMessage) {
	message = &EventMessage{}

	if msg != "" {
		// Message 第一行数据。
		fi := strings.Index(msg, "\r\n")
		if fi >= 0 {
			desc := msg[:fi]
			di := strings.Index(desc, ":")
			if di >= 0 {
				message.Description = desc[di+1:]
			}
			details := msg[fi+1:]
			message.Details = details
		}
	}

	return
}

func parseEventMap(pmap map[string]string, dstptr interface{}) error {
	dstv := reflect.ValueOf(dstptr)
	dstt := reflect.TypeOf(dstptr)
	if dstt.Kind() != reflect.Ptr ||
		dstt.Elem().Kind() == reflect.Ptr {
		err := fmt.Errorf("reflect failed: %s", "dest kind must be a struct pointer")
		return err
	}
	if dstv.IsNil() {
		err := fmt.Errorf("reflect failed: %s", "dest value cannot be nil")
		return err
	}

	dstve := dstv.Elem()

	etype := dstve.Type()
	num := dstve.NumField()

	for i := 0; i < num; i++ {
		// 获取反射类型信息
		structfield := etype.Field(i)
		name := structfield.Name

		// 通过 tag 获取属性值
		val := pmap[name]

		if val != "" {
			// 结构体属性信息
			field := dstve.Field(i)
			switch field.Kind() {
			case reflect.Struct:
				// 内嵌结构体不操作
				continue

			case reflect.String:
				dstve.FieldByName(name).SetString(val)

			case reflect.Int64:
				i, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					return fmt.Errorf("reflect failed: [%w]", err)
				}
				dstve.FieldByName(name).SetInt(i)

			default:
			}
		}
	}
	return nil
}
