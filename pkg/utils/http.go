package utils

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    
    "support-workflow/pkg/config"
    
    "github.com/larksuite/oapi-sdk-go/v3"
    larkauth "github.com/larksuite/oapi-sdk-go/v3/service/auth/v3"
)

type BasicAuth struct {
    username string
    password string
}

type Client struct {
    baseURL    string
    httpClient *http.Client
    authToken  string
    basicAuth  BasicAuth
}

func NewClient(baseURL string) *Client {
    return &Client{
        baseURL:    baseURL,
        httpClient: &http.Client{},
    }
}

func (c *Client) SetAuthToken(token string) {
    c.authToken = token
}

func (c *Client) Clone() *Client {
    return &Client{
        baseURL:    c.baseURL,
        httpClient: &http.Client{Timeout: c.httpClient.Timeout},
    }
}

func (c *Client) Post(path string, body interface{}) (*http.Response, error) {
    jsonBody, err := json.Marshal(body)
    if err != nil {
        return nil, fmt.Errorf("JSON序列化失败: %w", err)
    }
    
    req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewBuffer(jsonBody))
    if err != nil {
        return nil, fmt.Errorf("创建请求失败: %w", err)
    }
    
    req.Header.Set("Content-Type", "application/json")
    c.applyAuthHeaders(req)
    
    return c.httpClient.Do(req)
}

func (c *Client) Get(path string, respInst interface{}) error {
    req, err := http.NewRequest("GET", c.baseURL+path, nil)
    if err != nil {
        return fmt.Errorf("创建请求失败: %w", err)
    }
    
    c.applyAuthHeaders(req)
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    return c.parseResponse(resp, &respInst)
}

func (c *Client) parseResponse(resp *http.Response, respInst interface{}) error {
    defer resp.Body.Close()
    
    bodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("读取响应体失败: %w", err)
    }
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("请求失败，状态码: %d，响应: %s\n", resp.StatusCode, string(bodyBytes))
    }
    
    if err = json.Unmarshal(bodyBytes, &respInst); err != nil {
        return fmt.Errorf("JSON解析失败: %w", err)
    }
    return nil
}

func (c *Client) applyAuthHeaders(req *http.Request) {
    if c.authToken != "" {
        req.Header.Set("Authorization", "Bearer "+c.authToken)
    }
    
    if c.basicAuth.username != "" {
        req.SetBasicAuth(c.basicAuth.username, c.basicAuth.password)
    }
}

type SupportClient struct {
    Client
}

func NewSupportClient() SupportClient {
    conf := config.GetConf()
    return SupportClient{
        Client{
            baseURL: conf.SupportEndpoint, httpClient: &http.Client{},
            basicAuth: BasicAuth{
                username: conf.SupportUsername, password: conf.SupportPassword,
            },
        },
    }
}

type FeishuClient struct {
    Client *lark.Client
}

type AccessTokenResponse struct {
    AppAccessToken string `json:"app_access_token"`
}

func (c *FeishuClient) GetAccessToken() string {
    conf := config.GetConf()
    body := larkauth.NewInternalAppAccessTokenReqBodyBuilder().
        AppId(conf.FeishuAppID).AppSecret(conf.FeishuAppSecret).Build()
    req := larkauth.NewInternalAppAccessTokenReqBuilder().Body(body).Build()
    resp, err := c.Client.Auth.V3.AppAccessToken.Internal(context.Background(), req)
    if err != nil {
        return ""
    }
    if !resp.Success() {
        return ""
    }
    var response AccessTokenResponse
    if err = json.Unmarshal(resp.RawBody, &response); err != nil {
        fmt.Println("解析 AccessToken 失败: %w", err)
        return ""
    }
    return response.AppAccessToken
}

func NewFeishuClient() *FeishuClient {
    conf := config.GetConf()
    baseClient := lark.NewClient(conf.FeishuAppID, conf.FeishuAppSecret)
    return &FeishuClient{Client: baseClient}
}
