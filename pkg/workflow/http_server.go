package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"support-workflow/pkg/config"
	"support-workflow/pkg/utils"

	"github.com/gin-gonic/gin"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
)

type CompanyRequest struct {
	CompanyName string `json:"companyName"`
	ProductName string `json:"productName"`
}

type HttpServer struct {
	server *http.Server
	router *gin.Engine
}

func NewHttpServer() *HttpServer {
	conf := config.GetConf()
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./static")
	r.GET("/", index)
	r.POST("/companies", createCompany)

	return &HttpServer{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%v", conf.Port),
			Handler: r,
		},
		router: r,
	}
}

func (s *HttpServer) Start() error {
	log.Printf("HTTP服务器启动，监听端口 %v\n", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *HttpServer) Stop() {
	log.Println("正在优雅关闭HTTP服务器...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
}

func index(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

type TypeTextField struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Fields struct {
	Serial             int             `json:"编号"`
	CreatorName        string          `json:"交付负责人"`
	CompanyFullName    []TypeTextField `json:"客户全称"`
	ServiceStatus      string          `json:"服务状态"` // 暂时先不更新了，Support 是 bool 类型
	AbbreviatedName    string          `json:"简称"`
	ProductVersion     []TypeTextField `json:"系统版本"`
	SupportEndDate     int             `json:"维保结束时间"`
	Amount             string          `json:"规格"`
	StartDate          int             `json:"订阅开始时间"`
	EndDate            int             `json:"订阅结束时间"`
	SaleUser           string          `json:"销售"`
	MaintenanceRecords []TypeTextField `json:"维护记录"` // 维护记录
}

type Record struct {
	Fields   Fields `json:"fields"`
	RecordID string `json:"record_id"`
}

type FeishuResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		HasMore   bool     `json:"has_more"`
		Records   []Record `json:"items"`
		PageToken string   `json:"page_token"`
		Total     int      `json:"total"`
	} `json:"data"`
}

func GetMaxSerialFromFeishu() (int, error) {
	conf := config.GetConf()
	client := utils.NewFeishuClient()

	fieldName := "编号"
	//accessToken := client.GetAccessToken()
	body := larkbitable.NewSearchAppTableRecordReqBodyBuilder().
		FieldNames([]string{fieldName}).
		Sort([]*larkbitable.Sort{
			larkbitable.NewSortBuilder().FieldName(fieldName).Desc(true).Build(),
		}).Build()
	req := larkbitable.NewSearchAppTableRecordReqBuilder().
		PageSize(10).
		AppToken(conf.FeishuTableAppToken).
		TableId(conf.FeishuTableID).
		Body(body).Build()
	searchResp, err := client.Client.Bitable.V1.AppTableRecord.Search(
		//context.Background(), req, larkcore.WithUserAccessToken(accessToken), // TODO 这里尝试去掉 AccessToken 是否能访问
		context.Background(), req,
	)
	if err != nil {
		return 0, fmt.Errorf("获取企业编号失败: %w\n", err)
	}

	if !searchResp.Success() {
		return 0, fmt.Errorf("query serial failed: %s", larkcore.Prettify(searchResp.CodeError))
	}
	var response FeishuResponse
	if err = json.Unmarshal(searchResp.RawBody, &response); err != nil {
		return 0, fmt.Errorf("解析 Serial 响应体失败: %w", err)
	}
	maxNumber := 0
	for _, item := range response.Data.Records {
		if item.Fields.Serial > maxNumber {
			maxNumber = item.Fields.Serial
		}
	}
	return maxNumber, nil
}

func InsertRecordToFeishu(companyName string) (string, *larkbitable.AppTableRecord, error) {
	conf := config.GetConf()
	client := utils.NewFeishuClient()

	serial, err := GetMaxSerialFromFeishu()
	if err != nil {
		return "", nil, err
	}
	companySerial := serial + 1
	fullName := fmt.Sprintf("%d-%s", companySerial, companyName)
	insertReq := larkbitable.NewCreateAppTableRecordReqBuilder().
		AppToken(conf.FeishuTableAppToken).TableId(conf.FeishuTableID).
		AppTableRecord(larkbitable.NewAppTableRecordBuilder().
			Fields(map[string]interface{}{
				`最终客户名称`: fullName,
				`编号`:     companySerial,
				`客户全称`:   companyName,
			}).
			Build()).
		Build()

	insertResp, err := client.Client.Bitable.V1.AppTableRecord.Create(context.Background(), insertReq)
	if err != nil {
		return "", nil, err
	}
	if !insertResp.Success() {
		return "", nil, fmt.Errorf("insert row failed: %s", larkcore.Prettify(insertResp.CodeError))
	}
	return fullName, insertResp.Data.Record, nil
}

func createCompany(c *gin.Context) {
	companyReq := CompanyRequest{}
	if err := c.ShouldBindJSON(&companyReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conf := config.GetConf()
	webhookUrl := conf.WechatGroupRobotWebhook
	remindPhones := strings.Split(conf.RobotRemindsMobilePhones, ",")
	fullName, _, err := InsertRecordToFeishu(companyReq.CompanyName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	reqBody := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": fmt.Sprintf(
				"%s-%s-支持群", fullName, companyReq.ProductName,
			),
			"mentioned_mobile_list": remindPhones,
		},
	}

	client := utils.NewClient(webhookUrl)
	for i := 0; i < 5; i++ {
		resp, err := client.Post("", reqBody)
		if err == nil && resp.StatusCode == 200 {
			break
		}
		if i == 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Call wechat robot webhook failed."})
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "提交成功"})
}
