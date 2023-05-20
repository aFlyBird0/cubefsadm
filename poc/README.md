# CubeFS-Adm POC

本目录是 CubeFS-Adm 的概念验证（Proof of Concept）目录，用于在开发前期验证 CubeFS-Adm 的各项核心模块。

> 为什么要做 POC
> 1. 快速验证核心模块的可行性
> 2. 忽略部分不重要的细节
> 3. 每一个 POC 都是一个独立可运行/验证的模块，后续可以直接基于这个写单元测试

## 1. YAML 配置解析

> YAML 解析部分非常绕，后期会画流程图和举例。

curveadm 对配置文件中抽象出了 [层级、变量、replica](https://github.com/opencurve/curveadm/wiki/topology#%E7%89%B9%E6%80%A7) 三大特性。

其中前两者，层级（或者说全局配置覆盖局部配置），还有变量（我更愿意称之为函数），都是 adm 工具所不可或缺的。

本次 POC 以新的技术方案实现了这两个特性（replica 暂时不考虑，对于初版来说必要性不强，而且后期加上的话也很简单）。

## 1.1 最终效果

先讲一下两个特性综合后的最终效果（config内的字段只是举例，不是真实的配置）。

输入：

```yaml
meta_node_services:
  config:
    listen.ip: <<auto_add "port_a" 8080>>
  deploy:
    - host: server-host1
    - host: server-host2
```

输出：

```yaml
meta_node_services:
  deploy:
    - config:
        listen.ip: 8080
      host: server-host1
    - config:
        listen.ip: 8081
      host: server-host2
```

可以看到，输出中的每个 config 都继承了全局的 config，同时每个 config 中的listen.ip都被函数替换成了具体的值（port自动+1）。

后面来讲讲如何通过层级（复制）和变量（函数）实现这个效果。

### 1.2 层级（把全局配置应用到每个节点）

代码在 [yaml/override.go](./yaml/override/main.go) 中。

第一步，程序会读取所有的一级节点（包含 DataNode 等各种 Service），每个一级节点都包含了全局配置（config）和 deploy 列表，deploy 由 host（标识机器）和 config（局部配置）组成。

第一步的主要目的是，将全局配置（config）加到 deploy 中的每个 config 中（如果局部配置有同名字段，则覆盖全局配置）。

#### 1.2.1 效果

输入：

```yaml
meta_node_services:
  config:
    listen.ip: <<auto_add "port_a" 8080>>
  deploy:
    - host: server-host1
    - host: server-host2
mds_services:
  config:
    leader.electionTimeoutMs: 3
  deploy:
    - host: server-host1
      config:
        leader.electionTimeoutMs: 2
    - host: server-host2
    - host: server-host3

```

输出：

```yaml
meta_node_services:
  config: {}
  deploy:
    - host: server-host1
      config:
        listen.ip: <<auto_add "port_a" 8080>>
    - host: server-host2
      config:
        listen.ip: <<auto_add "port_a" 8080>>
mds_services:
  config: {}
  deploy:
    - host: server-host1
      config:
        leader.electionTimeoutMs: 2
    - host: server-host2
      config:
        leader.electionTimeoutMs: 3
    - host: server-host3
      config:
        leader.electionTimeoutMs: 3

```

## 1.3 变量（函数）

目前拿了最复杂的 auto_add 函数举例，该函数有两个参数，id 和 init_num。

当在同一个 YAML 区块中多次调用 id 相同的函数，会自动将 init_num 递增（左闭区间）。

### 1.3.1 效果

输入（上一步的输出）：

```yaml
meta_node_services:
  config: {}
  deploy:
    - host: server-host1
      config:
        listen.ip: <<auto_add "port_a" 8080>>
    - host: server-host2
      config:
        listen.ip: <<auto_add "port_a" 8080>>
mds_services:
  config: {}
  deploy:
    - host: server-host1
      config:
        leader.electionTimeoutMs: 2
    - host: server-host2
      config:
        leader.electionTimeoutMs: 3
    - host: server-host3
      config:
        leader.electionTimeoutMs: 3

```

输出：

```yaml
mds_services:
  config: {}
  deploy:
    - config:
        leader.electionTimeoutMs: 2
      host: server-host1
    - config:
        leader.electionTimeoutMs: 3
      host: server-host2
    - config:
        leader.electionTimeoutMs: 3
      host: server-host3
meta_node_services:
  config: {}
  deploy:
    - config:
        listen.ip: 8080
      host: server-host1
    - config:
        listen.ip: 8081
      host: server-host2

```

可以看到，第一步复制的时候，把函数声明（auto_add）也复制了，所以第二步的时候，函数被替换成了具体的值。

> 限制：目前 POC 版本有一定限制，即 config 下面的 key 只能设置拉平形式的 key，即只能设置 `listen.ip: 8080`，不能设置 `listen: {ip: 8080}`。

### 1.3.2 一些技术上的细节

**其他函数的实现？**

其实还有其他的一些函数，比如 service_host，可以用于自动获取当前主机名的 IP 地址。这个其实比 auto_add 简单，因为 auto_add 还需要通过闭包保存当前自增的值，而 service_host 只需要和主机列表简单比对一下就可以了。

**为什么使用 `<<` 和 `>>` 作为函数标识符？**

与 curveadm 不同，本项目没有通过正则来实现函数。正则写起来太复杂，而且容易出错，本着尽量不重复造轮子的原则，本项目直接基于 go template 来实现 YAML 解析。

而使用 `[[` 和 `{{` 作为函数标识符，会和 YAML 的语法冲突，所以只能使用 `<<` 和 `>>`。（第一步的时候，会先以 YAML 的方式解析、复制，第二步再以 go template 的方式解析函数并转回YAML，最后再转成真正的拓扑结构。所以要求每一步的输入都是合法的 YAML）

**为什么没有采用 curveadm 的 ${service_host_sequence} 语法？**

curveadm 示例中的这种语法，在文档中的应用场景主要是用端口自增，例如 `820${service_host_sequence}`。

但这样有很大的缺点：

1. 无法自定义起始值，比如从 8080 开始自增
2. 最多只能自增到 9，比如 8209，再往后就是 82010 了

本项目采用的自增函数，可以自定义起始值，同时支持无限自增。

**如何实现全局变量？**

代码和示例配置在 [yaml/funcWithVars/main.go](./yaml/funcWithVars/main.go) 中。

非常简单，例子如下：

```yaml
vars:
  init_port: 8080 # 关键配置
meta_node_services:
    config:
        listen.ip: <<auto_add "port_a" .init_port>> # 关键配置
    deploy:
        - host: server-host1
        - host: server-host2
other_servers:
  # ...
```

可以看到，我们可以在 YAML 中设置一个特殊的区块，用于存放全局变量（在第一步复制全局配置的时候读取）。

在第二步解析函数的时候，就能直接通过 `.` 来访问全局变量。

这是 Go Template 原生支持的（`.`开头的表示变量，非`.`开头的表示函数），技术上不需要做任何特殊处理和判断，非常简单。

甚至还支持在函数中引用全局变量，如上面的例子所示。

## 2. 远程控制 Docker

能够在运行 curveadm 的机器上，控制远程机器上的 Docker，是项目的核心技术支撑之一。

curveadm 采用的是通过 SSH 登录机器，然后手动执行 Docker 命令的方式。

本项目通过较为 Hack 的方式，实现了在不开启 Docker 远程 API 的情况下，通过 Go SDK 来控制远程机器上的 Docker（以获得原生的 Go 对象，免去了正则解析命令行输出的麻烦）。

该部分已经独立成了一个小库，方便其他社区引用与贡献。详见：[SSH-Container](https://github.com/aFlyBird0/ssh-container/tree/main)

## 3. CubeFS 服务拓扑解析与 机器列表配置

**关于机器列表配置：**

curveadm 在这块的配置很成熟，已经几乎是最佳实践，本项目没有做太多改动。详见：https://github.com/opencurve/curveadm/wiki/hosts#%E4%B8%BB%E6%9C%BA%E9%85%8D%E7%BD%AE

这块技术实现上比较简单，就是读取配置到结构体而已。不需要 POC。

**关于服务拓扑解析：**

在完成了前文提到的 YAML 解析之后，我们其实就已经得到了一个完整的服务拓扑的 YAML 配置文件，（所有的变量和函数已经解析，全局配置也全部复制到了对应的子层级中），可以直接映射成一个 Go 结构体。

所以这一步的工作，主要就是：

1. 重新读取 YAML，解析成 CubeFS 拓扑配置结构体
2. 根据自定义策略，设置默认值，生成部署所需的各种文件（ CubeFS Json 配置，Docker 配置等）。
3. 按一定的规则或顺序，在机器列表中通过 Docker SDK 创建容器，启动服务。


* 其中，CubeFS Json配置的生成，技术上可以通过 Go 结构体的 Marshal 方法来实现。
* Docker部署清单的生成，技术上可以通过 Go Template 来实现（用户无关，只在程序内部用）。

> 拓扑配置-> 部署文件 的生成，主要的复杂度在于字段的设计、默认值的设计、如何根据拓扑结构生成Json文件并处理好各种端口映射、挂载等细节。这块技术上一定是可以实现的，主要是需要慢慢调整和完善。

## 4. 生成了部署文件后，如何启动服务，CuebFS 配置文件保存在哪里？使用 Docker 还是 Docker Compose？

## 5. 节点扩容、缩容、服务重启、服务升级

## 6. 中间数据存储（是否需要数据库？）

## 其他细节


## 致谢

1. [curveadm](https://github.com/opencurve/curveadm) 是一个很成熟的 adm 工具，CuebFS 在用户交互设计上参考了 curveadm 的很多思，感谢 curveadm 的前期探索。不过技术实现思路， cubefsadm 几乎完全不同，例如 Docker 控制方式与 YAML 解析方案。希望二者能够共同进步，为用户提供更好的体验。
2. ChatGPT。本次 POC 开发，ChatGPT 在快速生成初版代码上提供了很大的帮助，大大加快了开发进度。
