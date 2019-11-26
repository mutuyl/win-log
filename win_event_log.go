package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// EventLogCache EventLog 缓存。
var EventLogCache map[int64]*EventLog

// EventLog 数据结构体。
type EventLog struct {
	EventID            int64
	MachineName        string
	Data               string
	Index              int64
	Category           string
	CategoryNumber     int64
	EntryType          string
	Source             string
	ReplacementStrings string
	InstanceId         int64
	TimeGenerated      string
	TimeWritten        string
	UserName           string
	Site               string
	Container          string
	Message            EventMessage
}

func (el EventLog) String() string {
	return fmt.Sprintf("%d\t%s\t%d\t%s\t\t%s\n", el.EventID, el.TimeGenerated, el.Index, el.Source, el.Message.Description)
}

// EventLog 列表排序。
type EventLogs []EventLog

func (els EventLogs) Len() int {
	return len(els)
}
func (els EventLogs) Less(i, j int) bool {
	return els[i].Index < els[j].Index
}
func (els EventLogs) Swap(i, j int) {
	els[i], els[j] = els[j], els[i]
}

// GetEventLog PowerShell 5.1 以前的版本，抓取日志信息。
func GetEventLog(beginTime, endTime string) {
	var out bytes.Buffer
	cmd := exec.Command("powershell", fmt.Sprintf(CmdEventLog, beginTime, endTime))
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("Command 'Get-EventLog' run: [%w]", err))
	}
	c, e := simplifiedchinese.GBK.NewDecoder().Bytes(out.Bytes())
	if e != nil {
		panic(fmt.Errorf("Decoding output: [%w]", e))
	}

	events, err := parseEventLog(string(c), "\r\n\r\n", "\r\n\r\n\r\n")
	if err != nil {
		panic(err)
	}
	els := EventLogs(events)
	sort.Sort(els)

	l := els.Len()

	// 清空缓存。
	EventLogCache = make(map[int64]*EventLog)

	if l > 0 {
		for i := 0; i < l; i++ {
			el := els[i]
			EventLogCache[el.Index] = &el

			// 传输到日志服务器。
			// 根据配置的 EventId 过滤。
			if cfg.WinEvtIds == "" {
				buf, err := json.Marshal(el)
				if err != nil {
					panic(err)
				}
				fmt.Println()
				logger.Info("%s", string(buf))
			} else {
				if strings.Contains(cfg.WinEvtIds, strconv.FormatInt(el.EventID, 10)) {
					buf, err := json.Marshal(el)
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

// parseEventLog 截取日志中 EventLog 数据部分。
func parseEventLog(data string, bes string, bee string) (events []EventLog, err error) {
	if data != "" {
		bsi := strings.Index(data, bes)
		bei := strings.Index(data, bee)
		if bsi >= 0 && bei >= 0 && bei >= bsi {
			data = data[bsi+len(bes) : bei]
		}
		if data == "" {
			return
		}

		sepID := "EventID"
		sepMessage := "Message"
		sepSource := "Source"
		items := strings.Split(data, "\r\n"+sepID)
		l := len(items)
		if l > 0 {
			events = make([]EventLog, 0)
			for _, v := range items {
				el := EventLog{}

				// 将 EventID 间隔字符串还原。
				// 生成完整的 EventLog 数据。
				v = sepID + v

				// 获取 Message 属性的索引。
				msgi := strings.Index(v, sepMessage)
				// 获取 Source 属性的索引。
				srci := strings.Index(v, sepSource)

				// Message 数据应该在索引 msgi 和 srci 之间的部分。
				if msgi >= 0 && srci >= 0 && srci > msgi {
					// 第一部分：EventID - Message
					evt1 := v[:msgi]
					// 第二部分：Message - Source
					msg2 := v[msgi:srci]
					message := parseEventMessage(msg2)
					el.Message = *message
					// 第三部分：Source - 最后
					evt3 := v[srci:]

					pmap := make(map[string]string)
					// 键值对属性部分日志。
					content := evt1 + evt3
					// 按行分解。
					kvs := strings.Split(content, "\r\n")
					for _, kv := range kvs {
						if kv != "" && strings.Contains(kv, ":") {
							bi := strings.Index(kv, ":")
							pmap[strings.TrimSpace(kv[:bi])] = strings.TrimSpace(kv[bi+1:])
						}
					}
					err = parseEventMap(pmap, &el)
					if err != nil {
						return nil, err
					}

					// 和上次缓存的 WinEvent 对比过滤。
					var elc *EventLog
					if EventLogCache != nil && len(EventLogCache) > 0 {
						elc = EventLogCache[el.Index]
					}
					if elc == nil {
						events = append(events, el)
					}

				}

			}
		}
	}
	return
}
