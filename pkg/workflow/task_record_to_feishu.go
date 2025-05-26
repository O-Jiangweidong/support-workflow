package workflow

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"support-workflow/pkg/config"
	"support-workflow/pkg/utils"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
)

const (
	MaintenanceRecordLastMarker = "MaintenanceRecordLastMarker"
	SplitFlag                   = "\n----------\n"
)

type MaintenanceRecordManage struct {
	records []MaintenanceRecord
	IDs     map[int]bool
}

func (mrm *MaintenanceRecordManage) Parse(records string) {
	for _, recordItem := range strings.Split(records, SplitFlag) {
		element := strings.SplitN(recordItem, "-", 2)
		if len(element) < 2 {
			continue
		}
		id, err := strconv.Atoi(element[0])
		if err != nil {
			continue
		}
		mrm.IDs[id] = true
	}
}

func (mrm *MaintenanceRecordManage) Exist(id int) bool {
	_, ok := mrm.IDs[id]
	return ok
}

type MaintenanceRecordTable struct {
	RecordID string `json:"-"`
	Content  string `json:"-"`
}

type MaintenanceRecord struct {
	ID                 int    `json:"id"`
	CompanyName        string `json:"clientName"`         // 客户名称
	MaintenanceTime    int    `json:"maintenanceTime"`    // 维护时间
	MaintenanceTypes   string `json:"maintenanceTypes"`   // 维护类型
	MaintenanceContext string `json:"maintenanceContext"` // 详细过程
	ModifiedByName     string `json:"modifiedByName"`     // 修改人
}

func (mr *MaintenanceRecord) String() string {
	t := time.Unix(0, int64(mr.MaintenanceTime)*int64(time.Millisecond))
	date := t.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
	return fmt.Sprintf(
		"%v-[%s]-[%s]-[%s]-[%s]", mr.ID, mr.MaintenanceTypes, date,
		mr.ModifiedByName, mr.MaintenanceContext,
	)
}

type MaintenanceRecordResponse struct {
	Data   []MaintenanceRecord `json:"data"`
	Marker int                 `json:"marker"`
}

type MaintenanceRecordToFeishuTask struct {
	executeTimes int
}

func (m *MaintenanceRecordToFeishuTask) getMaxValue() (maxValue int) {
	if m.executeTimes == 0 {
		maxValue = 5
	} else {
		maxValue = 1000
	}
	return
}

func (m *MaintenanceRecordToFeishuTask) getMaintenanceRecords() ([]MaintenanceRecord, error) {
	var maintenanceRecords []MaintenanceRecord
	var maintenanceRecordResp MaintenanceRecordResponse
	var marker int
	client := utils.NewSupportClient()
	cache := utils.GetCache()
	err := cache.Get(MaintenanceRecordLastMarker, &marker)
	if err != nil {
		marker = 0
	}
	for {
		maxValue := m.getMaxValue()
		baseUrl := "/openapi/v1/bi/maintenance-records?region=northern&max=%v"
		url := fmt.Sprintf(baseUrl, maxValue)
		if marker != 0 {
			url += fmt.Sprintf("&marker=%v", marker)
		}
		fmt.Println("URL: ", url)
		err = client.Get(url, &maintenanceRecordResp)
		if err != nil {
			return nil, err
		}
		m.executeTimes += 1
		marker = maintenanceRecordResp.Marker
		err = cache.Set(MaintenanceRecordLastMarker, marker, 0)
		maintenanceRecords = append(maintenanceRecords, maintenanceRecordResp.Data...)
		fmt.Println("Len: ", len(maintenanceRecords))
		if marker == -1 {
			break
		}
	}
	return maintenanceRecords, nil
}

func (m *MaintenanceRecordToFeishuTask) getFeishuMaintenanceRecord(companyName string) (*MaintenanceRecordTable, error) {
	instance, err := GetFeishuRecord(companyName)
	if err != nil {
		return nil, err
	}

	content := ""
	for _, f := range instance.Fields.MaintenanceRecords {
		content += f.Text
	}

	maintenanceTable := &MaintenanceRecordTable{
		RecordID: instance.RecordID, Content: content,
	}
	return maintenanceTable, nil
}

func (m *MaintenanceRecordToFeishuTask) updateDataToFeishu(mr MaintenanceRecord) error {
	conf := config.GetConf()
	client := utils.NewFeishuClient()
	feishuRecord, err := m.getFeishuMaintenanceRecord(mr.CompanyName)
	if err != nil {
		return fmt.Errorf("get %s record failed: %w", mr.CompanyName, err)
	}
	var newRecords []string
	var recordSet = make(map[int]bool)
	if feishuRecord.Content == "" {
		newRecords = append(newRecords, mr.String())
	} else {
		if mr.CompanyName == "宁夏医科大学" {
			fmt.Println(mr)
		}
		for _, item := range strings.Split(feishuRecord.Content, SplitFlag) {
			itemIDString := strings.Split(item, "-")[0]
			itemID, err := strconv.Atoi(itemIDString)
			if err != nil {
				continue
			}
			newRecords = append(newRecords, item)
			recordSet[itemID] = true
			if exists, _ := recordSet[mr.ID]; !exists {
				newRecords = append(newRecords, mr.String())
				recordSet[mr.ID] = true
			}
		}
	}

	req := larkbitable.NewUpdateAppTableRecordReqBuilder().
		AppToken(conf.FeishuTableAppToken).
		TableId(conf.FeishuTableID).
		RecordId(feishuRecord.RecordID).
		AppTableRecord(larkbitable.NewAppTableRecordBuilder().
			Fields(map[string]interface{}{
				`维护记录`: strings.Join(newRecords, SplitFlag),
			}).Build()).
		Build()

	resp, err := client.Client.Bitable.V1.AppTableRecord.Update(context.Background(), req)
	if err != nil {
		return fmt.Errorf("update %s failed: %w", mr.CompanyName, err)
	}

	if !resp.Success() {
		return fmt.Errorf("error response: %s", larkcore.Prettify(resp.CodeError))
	}
	log.Printf("Update maintenance record %v success", mr.CompanyName)
	return nil
}

func (m *MaintenanceRecordToFeishuTask) Execute() error {
	maintenanceRecords, err := m.getMaintenanceRecords()
	if err != nil {
		return err
	}
	for _, maintenanceRecord := range maintenanceRecords {
		err = m.updateDataToFeishu(maintenanceRecord)
		if err != nil {
			log.Printf("updating feishu maintenance record failed: %v", err)
		}
	}
	return nil
}
