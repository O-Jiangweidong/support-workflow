package workflow

import (
	"fmt"
	"time"

	"support-workflow/pkg/utils"
)

type SalesUser struct {
	Name string `json:"name"`
}

type Customer struct {
	Name            string `json:"name"`
	AbbreviatedName string `json:"abbreviatedName"` // 简称
}

type HumanTime int

func (t HumanTime) MarshalJSON() ([]byte, error) {
	tm := time.Unix(int64(t), 0)
	timeStr := tm.Format("2006-01-02 15:04:05")
	return []byte(fmt.Sprintf(`"%s"`, timeStr)), nil
}

type Subscription struct {
	Amount         int       `json:"amount"`         // 资产数量
	DeploymentTime HumanTime `json:"deploymentTime"` // 交付时间
	CreateTime     HumanTime `json:"createTime"`     // 创建时间
	StartDate      HumanTime `json:"startDate"`      // 订阅开始时间
	EndDate        HumanTime `json:"endDate"`        // 订阅结束时间
	SupportEndDate HumanTime `json:"supportEndDate"` // 维保结束时间
	SupportExpired bool      `json:"supportExpired"` // 维保状态
	Expired        bool      `json:"expired"`        // 服务状态
	Customer       Customer  `json:"client"`         // 客户信息
	SalesUser      SalesUser `json:"salesUser"`      // 销售信息
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
	CreatorName  string       `json:"creatorName"`     // 部署人/交付负责人
	ServiceType  string       `json:"serviceTypeName"` // 订阅类型
	Version      string       `json:"-"`
	DeployArch   string       `json:"-"`
	Subscription Subscription `json:"subscription"`
	OtherInfo    OtherInfo    `json:"content"`
}

func (m Maintenance) FitData() {
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

type SupportToFeishuTask struct {
}

func (m SupportToFeishuTask) Execute() error {
	return nil
}

func (m SupportToFeishuTask) Execute1() error {
	client := utils.NewSupportClient()
	productName := "JumpServer"
	var maintenanceInst MaintenanceResponse
	err := client.Get(fmt.Sprintf("/openapi/v1/bi/maintenances?product=%v", productName), maintenanceInst)
	if err != nil {
		return err
	}
	return nil
}
