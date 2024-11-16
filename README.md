<p align="center">
  <a href="https://github.com/wuzhengmao/gosync">
    <img src="doc/gosync.png" alt="sync - GO sync push service" width="240" /></a>
</p>

<p align="center">
  <a href="https://github.com/wuzhengmao/releases/latest"><img src="https://img.shields.io/github/v/release/wuzhengmao/gosync"/></a>
  <a href="https://raw.githubusercontent.com/v2fly/v2ray-core/master/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue"/></a>
  <a href="https://github.com/wuzhengmao/releases/latest"><img src="https://img.shields.io/github/downloads/wuzhengmao/gosync/total.svg"/></a>
</p>

## 特性

- 这是一个以推送变更为主的同步工具，远端需要启动rsyncd服务来接受更新
- 以GO语音开发，可以运行在不同架构的Linux平台上
- 监听本地目录的变化，并对可以归并的变更进行合并和剔除，提高同步的效率
- 每次启动工具可以先进行一次全量同步
- 支持禁止同步删除
- 支持失败重试，当失败队列超过阈值，可以触发全量同步
- 支持定时任务，可以灵活的定制一些策略，比如删除本地一周前的数据

## 依赖

- inotify 通常Linux发行版已经内置
- rsync 需安装，本地无需启动rsyncd服务

```bash
# debian / ubantu
apt-get install rsync
# centos / fedora / redhat
yum install rsync
```

## 安装

#### 二进制文件

<https://github.com/wuzhengmao/gosync/releases>

#### 源码编译

```bash
git clone https://github.com/wuzhengmao/gosync.git
make
cp dist/gosync /usr/local/bin
```

## 运行

#### 配置文件

配置文件名默认为gosync.yml，未使用-config参数指定时，默认按当前工作目录、执行文件目录、/etc、/etc/gosync的顺序搜索。

```yml
# gosync.yml
log:
  level: info                              # 日志等级：debug/info(default)/warn/error/fatal
  output: file                             # 日志输出：stdout(default)/syslog/file
  file:                                    # 使用文件日志时需要设置
    path: /var/log/gosync/gosync.log       # 日志文件路径
    max-size: 100                          # 单个日志文件大小限制(M)
    max-backups: 10                        # 日志备份数
    max-age: 30                            # 保留多少天日志
    compress: true                         # 压缩日志归档
rsync:
  host: 10.168.4.210                       # 远端rsyncd服务主机名或IP
  port: 873                                # 远端rsyncd服务端口，873为rsync协议的默认端口
  username: test                           # 连接远端rsyncd服务的用户名
  password: 123456                         # 连接远端rsyncd服务的密码
  timeout: 3                               # 连接远端rsyncd服务的超时时间(秒)
  space: hub                               # 对应远端rsyncd服务的模块
  root-path: /path/to/sync                 # 本地同步目录
  compress: false                          # 传输时是否启用压缩：true/false(default)
  allow-delete: false                      # 是否允许删除远端：true/false(default)
  full-sync: "startup"                     # 执行全量同步：startup(default 启动时执行)/none(不执行)/cron表达式(以定时任务的方式执行)
  excludes:                                # 配置排除同步的规则，示例中排除了vi产生的临时文件
    - "**/*.swp"
    - "**/*.swpx"
    - "**/4913"
queue:
  retry-interval: 2                        # 失败重试的时间间隔(秒)
  queue-capacity: 100                      # 同步队列的最大容量，超过这个容量会触发全量同步
jobs:
  - cron: "0 2 * * ?"                      # 定时任务执行时间，支持标准cron表达式，也支持@every/@after+?h?m?s的方式指定
    command: scripts/cleanup-7days-up.sh   # 可执行命令，运行的工作目录为配置文件所在目录
```

```bash
# run as app
gosync -config /etc/gosync/gosync.yml
# run as service
gosync -daemon -config /etc/gosync/gosync.yml
```

#### 安装服务

```bash
# create systemd service
curl -L https://raw.githubusercontent.com/wuzhengmao/gosync/refs/heads/main/systemd/gosync.service -o /etc/systemd/system/gosync.service
systemctl daemon-reload
# run gosync service
systemctl start gosync
# enable on startup
systemctl enable gosync
```

## 远端配置

#### 配置同步仓库

```bash
# /etc/rsyncd: configuration file for rsync daemon mode
# See rsyncd.conf man page for more options.

pid file = /var/run/rsyncd.pid
log file = /var/log/rsyncd.log
uid = root
gid = root
secrets file = /etc/rsyncd.secrets
transfer logging = yes

[hub]
path = /data/hub
list = yes
read only = no
write only = yes
dont compress = *.gz *.tgz *.zip *.z *.Z *.rpm *.deb *.bz2
auth users = test
hosts allow = 10.168.0.0/16
```

#### 运行rsyncd服务

```bash
# run rsyncd service
systemctl start rsyncd
# enable on startup
systemctl enable rsyncd
```
