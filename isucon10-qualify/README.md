# ICUSON10 qualify

[ISUCON10 予選マニュアル](https://gist.github.com/progfay/25edb2a9ede4ca478cb3e2422f1f12f6)

## Environment

Creating environment using AMI provided by [aws-isucon](https://github.com/matsuu/aws-isucon).

In this case we use `ami-03bbe60df80bdccc0`.

> 本来のサーバはCPU 2コア、メモリ1GBの3台構成です。

So we choose `t3.micro` (2c/1G) x3 and one `t3.medium` for benchmarking.

Remember to configure security rules for these machines. (Allow all inbound traffic in same security group)

`sudo -i -u isucon` to switch user.

`sudo su` to switch to root.

## Preparation

Check all the applications installed in server.
- Nginx (Listen 80, `systemctl status nginx`, `/etc/nginx/nginx.conf`)
- Go (Listen 1323)
- MySQL 5.7 (`/etc/mysql/mysqlconf.d/mysqld.cnf`)

Check frontend page. (SSH port forwarding)

First time benchmark:
```bash
isucon@ip-172-31-45-3:~/isuumo/bench$ ./bench --target-url http://172.31.36.183
2021/07/04 09:44:58 bench.go:78: === initialize ===
2021/07/04 09:45:05 bench.go:90: === verify ===
2021/07/04 09:45:06 bench.go:100: === validation ===
2021/07/04 09:45:33 load.go:181: 負荷レベルが上昇しました。
2021/07/04 09:45:58 load.go:181: 負荷レベルが上昇しました。
2021/07/04 09:45:59 fails.go:105: [client.(*Client).SearchEstatesNazotte] /home/isucon/isuumo/bench/client/webapp.go:367
    message("POST /api/estate/nazotte: リクエストに失敗しました")
[client.(*Client).Do] /home/isucon/isuumo/bench/client/client.go:136
    code(error timeout)
    *url.Error("Post \"http://172.31.36.183/api/estate/nazotte\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
    *http.httpError("context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
[CallStack]
    [client.(*Client).Do] /home/isucon/isuumo/bench/client/client.go:136
    [client.(*Client).SearchEstatesNazotte] /home/isucon/isuumo/bench/client/webapp.go:361
    [scenario.estateNazotteSearchScenario] /home/isucon/isuumo/bench/scenario/estateNazotteSearchScenario.go:214
    [scenario.runEstateNazotteSearchWorker] /home/isucon/isuumo/bench/scenario/load.go:100
    [runtime.goexit] /home/isucon/local/go/src/runtime/asm_amd64.s:1373
2021/07/04 09:46:00 fails.go:105: [client.(*Client).SearchEstatesNazotte] /home/isucon/isuumo/bench/client/webapp.go:367
    message("POST /api/estate/nazotte: リクエストに失敗しました")
[client.(*Client).Do] /home/isucon/isuumo/bench/client/client.go:136
    code(error timeout)
    *url.Error("Post \"http://172.31.36.183/api/estate/nazotte\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
    *http.httpError("context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
[CallStack]
    [client.(*Client).Do] /home/isucon/isuumo/bench/client/client.go:136
    [client.(*Client).SearchEstatesNazotte] /home/isucon/isuumo/bench/client/webapp.go:361
    [scenario.estateNazotteSearchScenario] /home/isucon/isuumo/bench/scenario/estateNazotteSearchScenario.go:214
    [scenario.runEstateNazotteSearchWorker] /home/isucon/isuumo/bench/scenario/load.go:100
    [runtime.goexit] /home/isucon/local/go/src/runtime/asm_amd64.s:1373
2021/07/04 09:46:06 bench.go:102: 最終的な負荷レベル: 2
{"pass":true,"score":646,"messages":[{"text":"POST /api/estate/nazotte: リクエストに失敗しました (タイムアウトしました)","count":2}],"reason":"OK","language":"go"}
```

Tools for analyze (Installed on local server):
- [GoAccess](https://goaccess.io/) (Nginx logs analyzer)
- [Soar](https://github.com/XiaoMi/soar) (SQL analyzer)

Analyze while benchmarking:
1. Nginx logs (getting latency of api)
  clean nginx log first: `cat /dev/null > /var/log/nginx/access.log` 
  ```
    access_log  /var/log/nginx/access.log  main;
  ```
  reload: `sudo nginx -s reload`
2. MySQL Slow sql logs
  ```
    slow_query_log         = 1
    slow_query_log_file    = /var/log/mysql/mysql-slow.log
    long_query_time = 1
  ```
  reload: `sudo systemctl restart mysql`
3. Golang pprof
  ```
    import (
        "net/http"
	      _ "net/http/pprof"
    )

    func main() {
        // pprof
        go http.ListenAndServe("127.0.0.1:9090", nil)
    }
  ```
4. Golang trace
  ```
  package main

  import (
    "fmt"
    "net/http"
    "os"
    "runtime/trace"
  )

  func init() {
    http.HandleFunc("/traceStart", traceStart)
    http.HandleFunc("/traceStop", traceStop)
  }

  func traces(w http.ResponseWriter, r *http.Request) {
    f, err := os.Create("trace.out")
    if err != nil {
      panic(err)
    }
    err = trace.Start(f)
    if err != nil {
      panic(err)
    }
    w.Write([]byte("TrancStart"))
    fmt.Println("StartTrancs")
  }

  func traceStop(w http.ResponseWriter, r *http.Request) {
    trace.Stop()
    w.Write([]byte("TrancStop"))
    fmt.Println("StopTrancs")
  }
  ```

Sync codes using SFTP in `/home/isucon/webapp/app`

Change golang systemd file:
```bash
isucon@ip-172-31-36-183:~/isuumo/webapp/app$ cat /etc/systemd/system/isuumo.go.service
[Unit]
Description=isuumo.go

[Service]
WorkingDirectory=/home/isucon/isuumo/webapp/app
EnvironmentFile=/home/isucon/env.sh
PIDFile=/home/isucon/isuumo/webapp/go/server.pid

User=isucon
Group=isucon
ExecStart=/home/isucon/isuumo/webapp/app/isuumo
ExecStop=/bin/kill -s QUIT $MAINPID

Restart   = always
Type      = simple

[Install]
WantedBy=multi-user.target
```

## Profiling

Do benchmark again.
```bash
go tool pprof http://127.0.0.1:1323/debug/pprof?debug=1
```

## Development

MySQL slow SQL logs on:
```
slow_query_log         = 1
slow_query_log_file    = /var/log/mysql/mysql-slow.log
```

