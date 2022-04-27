package main

import (
	"fmt"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	"github.com/tebeka/selenium"
	"io/ioutil"
	"jwzx-new-scrapy/tool"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

/*
	爬取服务:爬取当日通知，文字消息直接发送，文件则作为附件。支持通知时间自定义，若有则通知，若无则不通知。
	发邮件服务:使用第三方库实现邮件发送。提供路由上传需要服务的邮箱。
*/

const (
	chromeDriverPath = "D:/selenium/chromedriver.exe"
	jwzx             = "http://jwzx.cqupt.edu.cn"
	port             = 8039
)

func main() {
	cookies, err := seleniumLogin("https://ids.cqupt.edu.cn/authserver/login?service=https%3A%2F%2Fresource.cqupt.edu.cn%2Frump_frontend%2FloginFromCas%2F")
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println("cookie: ", cookies)
	tool.LinkRedis()
	//创建收集器
	c := colly.NewCollector()
	//创建一个新的收集器
	oc := c.Clone()

	//随机用户代理
	extensions.RandomUserAgent(oc)
	extensions.RandomUserAgent(c)
	//限速
	err = c.Limit(&colly.LimitRule{
		DomainGlob:  "jwzx.cqupt.edu.cn/*",
		Delay:       2 * time.Second,
		Parallelism: 5,
	})
	if err != nil {
		log.Println(err)
		return
	}

	c.OnError(func(res *colly.Response, err error) {
		log.Println("c-> ", "status:", res.StatusCode, " err:", err)
	})
	c.OnRequest(func(req *colly.Request) {
		//s := ""
		//for i := 0; i < len(cookies); i++ {
		//	s = s + cookies[i].Name + "=" + cookies[i].Value + ";"
		//}
		//fmt.Println(s)
		req.Headers.Set("cookie", "client_vpn_ticket="+cookies[0].Value+"; mLvnBZTNP4mtS=54rAH0_jGNuVItgG7U6Gj5EgYKaFuRWkFs8hoxtemGdOxK_G99zlLAPKiQQT4wnVk4q59ZOkz430UeKj0LvPnRA; PHPSESSID=p5aeres0qvdpb37494hjnamrjr; enable_undefined=true; mLvnBZTNP4mtT=YoCmi7YGxv7G9hTP7qTcjeI_EhvXhBhiYQOL4UjvbHXhsCmd7PcNl3WXGjI7Nvkq4Yryb2XLf1zxFTEoaoOOELkWnCsslNEceNqLz51nBSaYJJER_aQ15GniQr1GpBOzZmps1tdVR7bBLDtjtkz.0QR14Qy2klxQ1ls18R.pKAm8uEz0EeXOcv1U5o4CDCRDRbfHH_OHJHUMRErNtq.hXgdFbSrb3b5cdtOyPCu0z7KGkZQK.Pmp.dOhvjAwRho_HMzQVNXV.Nd4E4hiNbAbOcIhdVfuHud1whkzUq8rAbV5dnBTXg2xB3pSIcb7buenEqhUz0F8_fSrjggE0ehtH0rP93inDimlCcKPwDF3zzZ")
	})
	c.OnHTML("tbody", func(e0 *colly.HTMLElement) {
		path := e0.Request.URL.Path
		urlNums := 0 //记录连接数
		e0.ForEachWithBreak("tr", func(i int, e1 *colly.HTMLElement) bool {
			//爬取时间
			date := e1.ChildText("td>a+td")
			d, err := time.Parse("2006-01-02", date)
			if err != nil {
				log.Println(err)
				return true
			}
			//判断不为当日通知,退出循环遍历
			if !d.Equal(time.Now()) {
				return true
			}
			//爬取连接
			href := e1.Attr("td>a[href]")
			url := path + "/" + href
			//将连接保存至redis，设置有效时间为一天
			err = tool.SetUrl(i, url)
			if err != nil {
				log.Println(err)
				return true
			}
			urlNums = i + 1
			return true
		})
		//退出循环，再访问每个连接
		for i := 0; i < urlNums; i++ {
			//取出redis的连接
			getUrl, err := tool.GetUrl(i)
			if err != nil {
				log.Println(err)
				return
			}
			//访问这个连接
			oc.Visit(getUrl)
		}
	})

	oc.OnHTML("div[id='mainPanel']", func(e0 *colly.HTMLElement) {
		path := e0.Request.URL.Path
		//通知正文内容筛选
		content := e0.ChildText("p[style]+div")
		file1, err := os.Create("./source/" + time.Now().Format("2006-01-02") + "/" + "正文.txt")
		if err != nil {
			log.Println(err)
			return
		}

		file1.Write([]byte(content))
		file1.Close()
		//如有附件，保存硬盘
		if e0.ChildText("p>img") != "" {
			e0.ForEach("li>a[href]", func(i int, e1 *colly.HTMLElement) {
				//拼接连接
				href := e1.Attr("href")
				url := path + "/" + href
				//访问
				response, err := http.Get(url)
				if err != nil {
					log.Println(err)
					return
				}
				//匹配文档类型
				dis := response.Header.Get("Content-Disposition")
				//删除前缀
				f := strings.TrimPrefix(dis, "attachment; filename=")
				//创建文件与source文件夹
				file, err := os.Create("./source/" + time.Now().Format("2006-01-02") + "/" + f)
				if err != nil {
					log.Println(err)
					return
				}
				defer file.Close()
				//读入文件
				bytes, err := ioutil.ReadAll(response.Body)
				if err != nil {
					log.Println(err)
					return
				}
				_, err = file.Write(bytes)
				if err != nil {
					log.Println(err)
					return
				}
			})
		}

	})

	oc.OnError(func(res *colly.Response, err error) {
		log.Println("oc-> ", "status:", res.StatusCode, " err:", err)
	})
	c.Visit(jwzx)
}

func seleniumLogin(loginUrl string) ([]selenium.Cookie, error) {
	//在后台启动一个ChromeDriver实例
	driverService, err := selenium.NewChromeDriverService(chromeDriverPath, port, selenium.Output(os.Stderr))
	if err != nil {
		log.Println(err)
		return nil, err
	}

	defer driverService.Stop()

	// 创建新的远程客户端，这也将启动一个新会话。
	cap := selenium.Capabilities{"browserName": "chrome"}
	wd, err := selenium.NewRemote(cap, fmt.Sprintf("http://localhost:%s/wd/hub", strconv.Itoa(port)))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer wd.Quit()

	err = wd.Get(loginUrl)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	//填充账号
	element1, err := wd.FindElement(selenium.ByCSSSelector, ".m-account>div[class=\"username item\"]>#username")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	err = element1.SendKeys("1669277")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	//填充密码
	element2, err := wd.FindElement(selenium.ByCSSSelector, "#password")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	err = element2.SendKeys("7234nima@")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	//通过tag name进行定位
	element3, err := wd.FindElement(selenium.ByCSSSelector, "a[class='login-btn lang_text_ellipsis']")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	err = element3.Click()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	cookies, err := wd.GetCookies()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return cookies, nil
}

//将redis内的内容发送订阅的邮箱
func sendMail() {

}
