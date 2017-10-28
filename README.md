[![Build Status](https://travis-ci.org/shell909090/goproxy.png?branch=master)](https://travis-ci.org/shell909090/goproxy)

# Table of contents

* [Abstract](#abstract)
  * [Msocks](#msocks)
  * [Chnroutes](#chnroutes)
  * [Edns Client Subnet](#edns-client-subnet)
* [Install](#install)
  * [Binary](#binary)
  * [Debian Package](#debian-package)
  * [Docker Image](#docker-image)
* [Configure](#Configure)
  * [Cmdline Parameters](#cmdline-parameters)
  * [Config and Path](#config-and-path)
  * [Server Config](#server-config)
  * [Server Example](#server-example)
  * [HTTP Config](#http-config)
  * [HTTP Example](#http-example)
  * [Blackfile](#blackfile)
  * [Port Mapping](#port-mapping)
  * [Key Generation](#key-generation)
  * [Certification Config and Test](#certification-config-and-test)
  * [File Permission](#file-permission)
  * [Admin Interface](#admin-interface)
* [Compile](#compile)
* [Detail](#detail)
  * [Linux Kernel Setting](#linux-kernel-setting)
* [Thanks](#Thanks)

# Abstract

goproxy是基于go写的隧道代理服务器，主要用于翻墙。

主要分为两个部分，客户端和服务器端。客户端使用http协议向其他程序提供一个标准代理。当客户端接受请求后，会加密连接服务器端，请求服务器端连接目标。

具体工作细节是。首先查询国外DNS并获得正确结果(未污染结果)，然后把结果和IP段表对比。如果落在国内，直接代理。如果国外，多个请求复用一个加密tcp把请求转到国外vps上处理。

加密有两种模式，预共享密钥(PSK)和传输层安全协议(TLS)。首选推荐TLS模式。

PSK模式一般使用AES-CBC来加密数据，在服务器-客户端间预先共享一个key。在连接时互相交换IV。双方需要先保持16bytes的随机数用做密钥。这些随机数被base64编码放在key字段中。服务器和客户端需要保持一致。

## Msocks

msocks是类似于http2的封装协议，将多个数据流封装在一个tcp链接中。减少握手开销，降低模式被发现的可能性。但是由于多个tcp复用封装到一个tcp内，导致单tcp过慢时所有请求的速度都受到压制。因此记得调优tcp配置，增强LFN下的网络效率。而且注意，当高速下载境外资源时，其他翻墙访问会受到影响。

## Chnroutes

翻墙中经常需要对国内和国际地址分别处理，以获得最好的体验，或减少暴露。chnroutes是一个开源项目，从apnic世界范围的路由表信息中寻找属于中国的段，并对这些段采用直连。

## Edns Client Subnet

即使使用了chnroutes，在启用翻墙后，仍然可能发现国内网站速度受到影响，或者国内站变成国际站。这往往是因为DNS的关系。

正常情况下，每个站点都会配置一些区域镜像，以加速用户体验。当用户查询DNS时，用户查询递归服务器(一般属于用户所在的ISP)，递归服务器查询解析服务器(一般属于DNS托管商)。解析服务器会根据递归服务器的地址给予最恰当的回应。但注意，这里是根据"递归服务器"的地址，而不是用户的地址，因为解析服务器拿不到用户IP。

当我们翻墙时，是不能选择国内DNS的。因为不仅有DNS劫持，还有DNS投毒([DNS cache poisoning](https://zh.wikipedia.org/wiki/%E5%9F%9F%E5%90%8D%E6%9C%8D%E5%8A%A1%E5%99%A8%E7%BC%93%E5%AD%98%E6%B1%A1%E6%9F%93))。选择的比较多的一般是google dns(8.8.8.8)和OpenDNS(208.67.222.222)。但是选择国外服务器时，解析国内域名会变成一个国外IP试图访问国内网站。结果就是会分配一个对境外访问最快的镜像，或者干脆是国际站。

[EDNS](https://en.wikipedia.org/wiki/Extension_mechanisms_for_DNS)是一个在[rfc6891](https://tools.ietf.org/html/rfc6891)里规范的DNS扩展协议，其中有个字段client subnet在[rfc7871](https://tools.ietf.org/html/rfc7871)里规范了。这个字段允许递归服务器向解析服务器转发请求的时候，带上客户端IP地址。于是解析就能根据真正的地址而非"递归服务器地址"来给予回应。

换到我们这个场景下，仅仅递归和解析支持client subnet是不行的。因为我们用的递归服务器(例如8.8.8.8)依然不知道我们的客户端IP。因此需要在客户端和递归服务器之间支持edns client subnet。根据我的测试，google dns在部分区域支持(具体来说我只知道新加坡支持。估计因为anycast，各个地方命中到了不同的真实服务器)，google https dns全面支持。

# Install

## Binary

goproxy的最基础发行形态为二进制发行。整个程序包含一个bin文件和一个路由表(routes.list.gz)，放在任何一个目录下。启动时需要以-config参数指定配置文件，配置文件中需要指定路由表和各种文件的路径，相对路径需要以./开头。

## Debian Package

goproxy可以编译为deb包，此种形态下可以直接安装到debian基础的系统中。路由表文件会被安装到/usr/share/goproxy/routes.list.gz，配置文件路径为/etc/goproxy。启动时默认为root，日志文件为/var/log/goproxy.log，没有logrotate。

## Docker Image

TODO

# Configure

## Cmdline Parameters

命令行接收-config参数来制定配置文件。

## Config and Path

系统默认使用/etc/goproxy/config.json作为配置文件，这一路径可以通过命令行参数-config来修改。

配置文件内使用json格式，其中可以指定以下内容：

* mode: 运行模式，可以为server/http/留空。留空是个特殊模式，表示不要启动。
* listen: 监听地址，一般是:port，表示监听所有interface的该端口。
* logfile: log文件路径，留空表示输出到stdout。在deb包中建议留空，用init脚本的机制来生成日志文件。
* loglevel: 日志级别，必须设定。支持EMERG/ALERT/CRIT/ERROR/WARNING/NOTICE/INFO/DEBUG。
* adminiface: 服务器端的控制端口，可以看到服务器端有多少个连接，分别是谁。
* dnsnet: dns的网络模式，支持四个选项，udp/tcp/https/internal。默认为udp模式，可选用tcp模式。设定为https采用google dns-over-https。以上三种均为直接连接。使用internal模式时，dns查询和回复会被搭载到msocks的连接上，发给服务器完成。internal模式仅能在client采用，服务器端仅采用https模式。因为只有https模式支持edns-client-subnet功能。
* dnsaddrs: dns查询的目标地址列表。如不定义则采用系统自带的dns系统，会读取默认配置并使用。

在服务器模式和http模式下各有一些额外项目可配置，这些配置和上面的配置是平级的。

## Server Config

服务器模式运行在境外机器上，监听某个端口提供服务。客户端可以连接服务器端，通过他连接目标tcp。

* cryptmode: 字符串。tls表示使用tls模式，其他表示使用PSK模式。
* rootcas: 字符串，只在tls模式下生效。以回车分割的多行字符串，每行一个文件路径，表示服务器认可的客户端ca根。不设定的话服务器端不做客户端证书验证。
* certfile: 字符串，只在tls模式下生效。服务器端使用的证书文件。
* certkeyfile: 字符串，只在tls模式下生效。服务器端使用的证书密钥。
* forceipv4: 布尔型。是否强制任何拨号都使用ipv4。
* cipher: 加密算法，只在PSK模式下生效。可以为aes/des/tripledes，默认aes。
* key: 密钥，只在PSK模式下生效。16个随机数据base64后的结果，客户端必须严格匹配方能通讯。
* auth: dict类型。认证用户名/密码对。不设定表示不验证用户。

## Server Example

	{
		"mode": "server",
		"listen": ":5233",
	 
		"logfile": "",
		"loglevel": "WARNING",
		"adminiface": "127.0.0.1:5234"

	    "forceipv4": true,
	    "cryptmode": "tls",
	    "rootcas": "./ca.crt",
	    "certfile": "./fullchain.pem",
	    "certkeyfile": "./privkey.pem"
	}

## HTTP Config

http模式运行在本地，需要一个境外的server服务器做支撑，对内提供http代理。

* blackfile: 黑名单文件，http模式下可选。
* minsess: 最小session数，默认为1。
* maxconn: 一个session的最大connection数，超过这个数值会启动新session。默认为64。
* servers: 服务器列表。
* httpuser: 客户端访问此http代理服务时的用户名。表示需要验证客户端身份。
* httppassword: 客户端访问此http代理服务时的密码。
* portmaps: 端口映射配置，将本地端口映射到远程任意一个端口。
* dnserver: 一个UDP端口。在此端口提供dns服务。服务会通过dnsnet里设定的模式去查询。此功能尚未提供。

其中servers是一个列表，成员定义如下：

* server: 中间代理服务器地址。
* cryptmode: 字符串。tls表示使用tls模式，其他表示使用PSK模式。
* rootcas: 字符串，只在tls模式下生效。以回车分割的多行字符串，每行一个文件路径，表示客户认可的服务器端ca根。不设定的话使用系统根证书设定。
* certfile: 字符串，只在tls模式下生效。客户端使用的证书文件。
* certkeyfile: 字符串，只在tls模式下生效。客户端使用的证书密钥。
* cipher: 加密算法，PSK下生效。可以为aes/des/tripledes。默认为aes。
* key: 密钥，PSK下生效。16个随机数据base64后的结果。
* username: 连接用户名。
* password: 连接密码。

其中portmaps的配置应当是一个列表，每个成员都应设定如下的值。

* net: 映射模式，支持tcp/tcp4/tcp6/udp/udp4/udp6。注意：6没测试过。
* src: 源地址。
* dst: 目标地址。

## HTTP Example

	{
		"mode": "http",
		"listen": ":5233",
	 
		"loglevel": "WARNING",
		"adminiface": "127.0.0.1:5234"

		"dnsnet": "internal",
		"blackfile": "/usr/share/goproxy/routes.list.gz",

        "servers": [
		    {
			    "cryptmode": "tls",
			    "server": "srv:5233",
			    "rootcas": "./ca.crt",
			    "certfile": "./client.crt",
			    "certkeyfile": "./client.key"
			}
		]
	}

## Blackfile

黑名单文件是一个路由文件，其中列出的子网将不会由服务器端代理，而是直接连接。这通常用于部分IP不希望通过服务器端的时候。

黑名单文件使用文本格式，每个子网一行。行内以空格分割，第一段为IP地址，第二段为子网掩码。允许使用gzip压缩，后缀名必须为gz，可以直接读取。routes.list.gz为样例。

CIDR style ip range definition is acceptable.

## port mapping

通过portmaps项，可以将本地的tcp/udp端口转发到远程任意端口。

注意：尚未测试。

## key generation

可以使用以下语句生成，写入两边的config即可。

    head -c 16 /dev/random | base64

## Certification Config and Test

推荐模式下，goproxy走的是标准TLS验证流程。配置模式是，服务器持有的CA可以验证客户端的cert和key，客户端持有的CA可以验证服务器端的cert和key。并且，我强烈的建议你为服务器端配置一个合法公开签署的证书——就是正常给网站配置https用的那种。因为自签署的证书容易被发现并识别。

在这个前提下，客户端需要持有服务器的CA根（注意，一定要找到对应的服务器颁发者的CA根，因为我不相信很多系统里内置的CA，里面有一些你懂，可能发生MITM攻击），服务器则持有被颁发的cert和key。客户端则没有这个要求，你可以自己生成一个CA，证书让服务器持有，然后颁发证书给客户端。

在这个过程中，你可能需要诊断PKI体系配置是否正确。最简单的办法是用openssl来验证。在上述模式下，如果你配置正确的话，你可以用这句语句来连接服务器端。

	openssl s_client --showcerts -cert client.crt -key client.crt --connect serverip:port

注意，我禁用了TLS1.2以外的所有协议，并且只允许部分cipher。所以，如果你是自己编写代码去连接的话，注意协议和cipher协商。

## File Permission

goproxy可以使用nobody和nogroup作为启动用户和组。这是一个非常小权限的组，在系统内相对比较安全。

但是在TLS模式下，goproxy需要读取证书文件。这些文件（尤其是key）出于安全理由，往往都指定为root读写，其他人没有权限。因此debian包往往在启动时直接制定用户使用root跑。如果你需要换回nobody，请修改/lib/systemd/system/goproxy.service，去掉注释。然后再用`systemctl daemon-reload`重新加载配置，用`systemctl restart goproxy`重启服务。

# Compile

## deb包解说

deb包是适用于debian/ubuntu的安装包。目前打包和测试都是在debian testing上完成，因此对此种系统的支持最完美。debian stable上可保证正常运行。ubuntu的兼容性希望得到反馈。同时希望有人做ubuntu移植，将启动模式改为upstart。

deb包中，主程序在/usr/bin下，启动项在/etc/init.d/goproxy下，配置文件在/etc/goproxy下。修改配置文件后重启服务生效。

默认black文件在/usr/share/goproxy/routes.list.gz。日志默认在/var/log/goproxy.log生成。日志的配置在init文件中修改。

在debian目录下有个默认的init脚本，负责将goproxy封装为服务。

## tar包解说

tar包内包含主程序，routes.list.gz示例。没有config.json示例。因此你需要自行编写一个正确的config.json，然后使用goproxy -config config.json来启动程序。

整个包不需要安装，手工启动和关闭。如果需要自动启动机制，请自行处理。

## docker

build "gobuilder" image

	cd gobuilder
	./build.sh

build debian package

	sudo docker run --rm -v "$PWD":/srv/myapp/ -w /srv/myapp/ gobuilder make build-deb

result can be found in debuild directory. take care of the permission.

# Detail

## Linux Kernel Setting

	net.ipv4.tcp_congestion_control = bbr
	net.ipv4.tcp_retries2 = 8
	net.core.rmem_default = 2621440
	net.core.rmem_max = 16777216
	net.core.wmem_default = 655360
	net.core.wmem_max = 16777216
	net.ipv4.tcp_rmem = 4096        2621440 16777216
	net.ipv4.tcp_wmem = 4096        655360  16777216

主要是增加吞吐。含义为如下:

* 使用bbr作为拥塞控制协议（非常重要，尤其是对服务器端非常有效）。
* tcp重传次数设定为8。由于msocks并没有检测远端是否收到了数据（tcp保证这一点），因此当远端消失时，是由tcp的重传失败机制来废弃连接。这个机制默认需要924.6秒以上来断开连接，而未断开的连接在这种状态下都会形同僵死，因此实际中我们需要将他调快一点。根据RFC1122的建议，最低不应少于100秒，对应值为8。更多说明请查看[这里](https://www.kernel.org/doc/Documentation/networking/ip-sysctl.txt)。
* 调整网络收发缓冲区的大小。

## 连接池规则

在msocks的客户端，一次会主动发起一个连接。当连接数低于一定个数时会主动补充(目前编译时设定为1)。

在连接时，会寻找承载tcp最少的一根去用。如果所有连接中，承载tcp最小的连接数大于一定值(配置中的maxconn)，那么会在后台再增加一根tcp（不影响当前连接的选择）。

当msocks连接断开时，在上面承载的tcp不会主动迁移到其他msocks上，而是会跟着断开。如果连接池满足一定规则(如上所述)，那么断开的连接会重新发起。

连接池不会主动释放链接。但是在断开时不满足规则的链接不会被重建。这使得连接池可以借助链接的主动断开回收msocks连接。

总体来说，连接池使得每个tcp承载的最大连接数保持在一定值。避免大量连接堵塞在一个tcp上，同时也尽力避免频繁的tcp连接握手和释放。

## 服务器选择规则

当链接数不足时，会发起新连接。由于配置允许写入多个服务器端，因此程序会随机选择一个配置尝试连接。如果尝试失败（无法握手或者超时），会选择下一个配置。如此重复两轮，如果都无法连接，则连接发起失败。

# Thanks

* 路由表来自[chnroutes](https://github.com/fivesheep/chnroutes)项目。

在此表示感谢

# TODO

* Found out why connection always blocked.
* Enable and Disable servers
* 增加dns对外服务？
* Encapsulate tcp into http.
* Speed control, low speed go first?
