package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"support-workflow/pkg/config"
	"support-workflow/pkg/utils"

	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
)

type SalesUser struct {
	Name string `json:"name"`
}

type Customer struct {
	Name            string `json:"name"`
	AbbreviatedName string `json:"abbreviatedName"` // 简称
}

type Subscription struct {
	Amount         int       `json:"amount"`          // 资产数量
	DeploymentTime int       `json:"deploymentTime"`  // 交付时间
	ServiceType    string    `json:"serviceTypeName"` // 订阅类型
	StartDate      int       `json:"startDate"`       // 订阅开始时间
	EndDate        int       `json:"endDate"`         // 订阅结束时间
	SupportEndDate int       `json:"supportEndDate"`  // 维保结束时间
	Expired        bool      `json:"expired"`         // 服务状态
	Customer       Customer  `json:"client"`          // 客户信息
	SalesUser      SalesUser `json:"salesUser"`       // 销售信息
}

type ContentMap struct {
	Value1 string `json:"value1"`
}

type Element struct {
	Title      string     `json:"title"`
	ContentMap ContentMap `json:"contentMap"`
}

type OtherInfo struct {
	Elements []Element `json:"elements"`
}

type Maintenance struct {
	ID           int          `json:"id"`
	RecordID     string       `json:"-"`
	Serial       int          `json:"-"`
	CreatorName  string       `json:"creatorName"` // 部署人/交付负责人
	Version      string       `json:"-"`           // 产品版本
	DeployArch   string       `json:"-"`           // 部署架构
	Subscription Subscription `json:"subscription"`
	OtherInfo    OtherInfo    `json:"content"`
}

func (m *Maintenance) Same(other *Maintenance) bool {
	if other.CreatorName != m.CreatorName {
		return false
	}
	if other.Version != m.Version {
		return false
	}
	if other.Subscription.Amount != m.Subscription.Amount {
		return false
	}
	if other.Subscription.StartDate != m.Subscription.StartDate {
		return false
	}
	if other.Subscription.EndDate != m.Subscription.EndDate {
		return false
	}
	if other.Subscription.SalesUser.Name != m.Subscription.SalesUser.Name {
		return false
	}
	if other.Subscription.Customer.Name != m.Subscription.Customer.Name {
		return false
	}
	if other.Subscription.Customer.AbbreviatedName != m.Subscription.Customer.AbbreviatedName {
		return false
	}
	if other.Subscription.SupportEndDate != m.Subscription.SupportEndDate {
		return false
	}
	return true
}

func (m *Maintenance) FitData() {
	for _, ele := range m.OtherInfo.Elements {
		if ele.Title == "version" {
			m.Version = ele.ContentMap.Value1
		} else if ele.Title == "form1prop11" {
			m.DeployArch = ele.ContentMap.Value1
		}
	}
}

type MaintenanceResponse struct {
	Data   []Maintenance `json:"data"`
	Marker int           `json:"marker"`
}

type MaintenanceToFeishuTask struct {
	productName  string
	executeTimes int
	maxValue     int
}

func (m *MaintenanceToFeishuTask) getMaintenances() ([]Maintenance, error) {
	var maintenances []Maintenance
	var maintenanceResp MaintenanceResponse
	client := utils.NewSupportClient()
	for {
		url := fmt.Sprintf("/openapi/v1/bi/maintenances?region=northern&product=%v&max=%v", m.productName, m.maxValue)
		if maintenanceResp.Marker != 0 {
			url += fmt.Sprintf("&marker=%v", maintenanceResp.Marker)
		}
		fmt.Println("URL: ", url)
		err := client.Get(url, &maintenanceResp)
		if err != nil {
			return nil, err
		}
		m.executeTimes += 1
		maintenances = append(maintenances, maintenanceResp.Data...)
		fmt.Println("Len: ", len(maintenances))
		if maintenanceResp.Marker == -1 {
			break
		}
	}
	return maintenances, nil
}

func (m *MaintenanceToFeishuTask) getFeishuMaintenance(companyName string) (*Maintenance, error) {
	instance, err := GetFeishuRecord(companyName)
	if err != nil {
		return nil, err
	}

	var version string
	amount, _ := strconv.Atoi(instance.Fields.Amount)
	if len(instance.Fields.ProductVersion) > 0 {
		version = instance.Fields.ProductVersion[0].Text
	}
	maintenance := &Maintenance{
		Serial:      instance.Fields.Serial,
		RecordID:    instance.RecordID,
		CreatorName: instance.Fields.CreatorName,
		Version:     version,
		Subscription: Subscription{
			StartDate: instance.Fields.StartDate,
			EndDate:   instance.Fields.EndDate,
			Amount:    amount,
			SalesUser: SalesUser{
				Name: instance.Fields.SaleUser,
			},
			Customer: Customer{
				Name:            instance.Fields.CompanyFullName[0].Text,
				AbbreviatedName: instance.Fields.AbbreviatedName,
			},
			SupportEndDate: instance.Fields.SupportEndDate,
		},
	}
	return maintenance, nil
}

func (m *MaintenanceToFeishuTask) updateOrCreateFeishuRecord(maintenance Maintenance) error {
	conf := config.GetConf()
	client := utils.NewFeishuClient()
	companyName := maintenance.Subscription.Customer.Name
	if companyName == "" {
		return nil
	}
	feishuMaintenance, err := m.getFeishuMaintenance(companyName)
	if err != nil {
		return fmt.Errorf("update %s failed: %w", companyName, err)
	}

	abbreviatedName := maintenance.Subscription.Customer.AbbreviatedName
	record := larkbitable.NewAppTableRecordBuilder().
		Fields(map[string]interface{}{
			`最终客户名称`: fmt.Sprintf("%v-%s", feishuMaintenance.Serial, abbreviatedName),
			`客户全称`:   companyName,
			`简称`:     abbreviatedName,
			`销售`:     maintenance.Subscription.SalesUser.Name,
			`交付负责人`:  maintenance.CreatorName,
			`系统版本`:   maintenance.Version,
			`部署架构`:   maintenance.DeployArch,
			`订阅类型`:   maintenance.Subscription.ServiceType,
			`规格`:     strconv.Itoa(maintenance.Subscription.Amount),
			`订阅开始时间`: maintenance.Subscription.StartDate,
			`订阅结束时间`: maintenance.Subscription.EndDate,
			`维保结束时间`: maintenance.Subscription.SupportEndDate,
		}).Build()
	if feishuMaintenance.RecordID == "" {
		req := larkbitable.NewCreateAppTableRecordReqBuilder().
			AppToken(conf.FeishuTableAppToken).
			TableId(conf.FeishuTableID).
			AppTableRecord(record).Build()
		resp, err := client.Client.Bitable.V1.AppTableRecord.Create(context.Background(), req)
		if err != nil {
			return fmt.Errorf("create %s failed: %w", companyName, err)
		}
		if !resp.Success() {
			return fmt.Errorf("error response: %s", resp.RawBody)
		}
		log.Printf("Create maintenance %v success", companyName)
	} else if !maintenance.Same(feishuMaintenance) {
		req := larkbitable.NewUpdateAppTableRecordReqBuilder().
			AppToken(conf.FeishuTableAppToken).
			TableId(conf.FeishuTableID).
			RecordId(feishuMaintenance.RecordID).
			AppTableRecord(record).Build()

		resp, err := client.Client.Bitable.V1.AppTableRecord.Update(context.Background(), req)
		if err != nil {
			return fmt.Errorf("update %s failed: %w", companyName, err)
		}

		if !resp.Success() {
			return fmt.Errorf("error response: %s", resp.RawBody)
		}
		log.Printf("Update maintenance %v success", companyName)
	}
	return nil
}

func (m *MaintenanceToFeishuTask) Execute() error {
	maintenances, err := m.getMaintenances()
	if err != nil {
		return err
	}
	for _, maintenance := range maintenances {
		maintenance.FitData()
		err = m.updateOrCreateFeishuRecord(maintenance)
		if err != nil {
			log.Printf("Error updating feishu maintenance: %v", err)
		}
	}
	m.sendMsgToWecom()
	return nil
}

func (m *MaintenanceToFeishuTask) sendMsgToWecom() {
	conf := config.GetConf()
	webhookUrl := conf.WechatMessageRobotWebhook
	now := time.Now()
	currentTime := now.Format("2006-01-02 15:04:05")
	reqBody := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": fmt.Sprintf(
				"[%s] 完成 Support 门户客户记录同步", currentTime,
			),
		},
	}
	client := utils.NewClient(webhookUrl)
	for i := 0; i < 5; i++ {
		resp, err := client.Post("", reqBody)
		if err == nil && resp.StatusCode == 200 {
			break
		}
		if i == 4 {
			log.Fatalf("Send message to wecom failed: %v", resp)
		}
	}

}

func GetFeishuRecord(companyName string) (*Record, error) {
	conf := config.GetConf()
	client := utils.NewFeishuClient()
	req := larkbitable.NewSearchAppTableRecordReqBuilder().
		AppToken(conf.FeishuTableAppToken).
		TableId(conf.FeishuTableID).
		Body(larkbitable.NewSearchAppTableRecordReqBodyBuilder().
			Filter(larkbitable.NewFilterInfoBuilder().
				Conjunction(`and`).
				Conditions([]*larkbitable.Condition{
					larkbitable.NewConditionBuilder().
						FieldName(`客户全称`).
						Operator(`is`).
						Value([]string{companyName}).
						Build(),
				}).Build()).
			Build()).
		Build()

	resp, err := client.Client.Bitable.V1.AppTableRecord.Search(context.Background(), req)
	if err != nil {
		return nil, err
	}

	if !resp.Success() {
		return nil, fmt.Errorf("exist request failed: %s", resp.RawBody)
	}
	var instResp FeishuResponse
	if err = json.Unmarshal(resp.RawBody, &instResp); err != nil {
		return nil, fmt.Errorf("解析表格中是否存在实施记录失败: %w", err)
	}
	if len(instResp.Data.Records) < 1 {
		record := Record{
			Fields: Fields{
				Serial: 0,
				CompanyFullName: []TypeTextField{
					{Type: "", Text: companyName},
				},
			},
		}
		return &record, nil
	}
	return &instResp.Data.Records[0], nil
}
