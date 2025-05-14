package workflow

import (
	"context"
	"fmt"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
	"log"
	"net/http"
	"strings"
	"time"

	"support-workflow/pkg/config"
	"support-workflow/pkg/utils"

	"github.com/gin-gonic/gin"
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

type SerialResponse struct {

}

func getCompanySerial() (int, error) {
	conf := config.GetConf()
	client := utils.NewFeishuClient()

	body := larkbitable.NewSearchAppTableRecordReqBodyBuilder().FieldNames([]string{"编号"}).Build()
	req := larkbitable.NewSearchAppTableRecordReqBuilder().
		AppToken(conf.FeishuTableAppToken).TableId(conf.FeishuTableID).Body(body).Build()
	resp, err := client.Client.Bitable.V1.AppTableRecord.Search(
		context.Background(), req,
		larkcore.WithUserAccessToken(client.GetAccessToken()),
	)
	if err != nil {
		return 0, fmt.Errorf("获取企业编号失败: %w\n", err)
	}

	// 服务端错误处理
	if !resp.Success() {
		return 0, fmt.Errorf("logId: %s, error response: \n%s", resp.RequestId(), larkcore.Prettify(resp.CodeError)
	}
	return 0, nil
}

func createCompany(c *gin.Context) {
	request := CompanyRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conf := config.GetConf()
	webhookUrl := conf.WechatGroupRobotWebhook
	remindPhones := strings.Split(conf.RobotRemindsMobilePhones, ",")
	companySerial, err := getCompanySerial()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	reqBody := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": fmt.Sprintf(
				"%v-%v-%s-支持群", companySerial, request.CompanyName, request.ProductName,
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
