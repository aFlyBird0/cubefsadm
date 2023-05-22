# CubeFS-Adm POC

本目录是 CubeFS-Adm 的概念验证（Proof of Concept）目录，用于在开发前期验证 CubeFS-Adm 的各项核心模块。

> 为什么要做 POC
> 1. 快速验证核心模块的可行性
> 2. 忽略部分不重要的细节
> 3. 每一个 POC 都是一个独立可运行/验证的模块，后续可以直接基于这个写单元测试

## 0. 产品最终效果

大概描述一下用户如何去使用 CubeFS-Adm。

首先这是一个 CLI，拥有两个核心配置文件，一个配置是主机列表，另一个配置是集群拓扑配置，后者会引用前者的主机信息。

* 对集群的任何"写"（扩容、修改、删除等）操作，都是一个 `apply` 命令，附上最新的期望的集群拓扑配置，adm 会自动计算出需要的变更，然后执行。
* 对集群的读操作，也是一个核心的命令。名字可能是 `get` `list` `read` 等。

## 1. YAML 配置解析

主要是设计集群拓扑文件的特殊语法，比如快速生成多个节点的配置，以及设置 Port 的自增等。

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

### 1.2.2 关于 Replica

replica 的作用主要是，在同一个主机下部署多个相同角色的服务。可以考虑在 deploy 中增加一个 replica 字段，表示这个服务的副本数。

然后在将全局配置应用到每个节点的局部配置前，先根据 replica 字段，将 deploy 列表中的每个节点复制若干份。

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

其实 CuebFS 组件的部署，主要包含了 Docker 相关的配置（镜像、端口、挂载等），以及 CubeFS 本身的配置（Json 文件）。

同时配置又会主要分为两种，一种是主动设置的（比如设置 master 运行在哪个端口），一种是基于用户填写的已有配置自动生成的（比如 DataNode 需要配置 master 的地址数组）。

所以主要是设置必要的 Docker 部署字段，以及将 Json 中有必要设置的字段暴露出来。其余的采取默认值+自动生成的方式。

## 4. CuebFS 配置文件保存在哪里？生成了部署文件后，如何启动服务，使用 Docker 还是 Docker Compose？

首先，每个服务的配置文件的存放位置，没有必要去让用户主动修改，定死就好。用户只需要选定一个根目录，比如 ~/.cubefsadm，内部的目录结构就由程序自动生成（固定）。

目前会有这样一些文件：

1. 主机列表文件和集群拓扑文件。完全开放给用户修改，可以放在任何位置，只要在启动时指定配置文件地址（文件夹）即可。
2. 上一次的渲染过的集群拓扑文件。这个文件是程序自动生成的，不需要用户修改，放在 $cubefsadm/last-deployed/ 下。
3. Docker 部署清单文件。这个文件是程序自动生成的，不需要用户修改，放在 $cubefsadm/deploy/ 下。
  1. deploy 文件夹下，按主机名区分不同的主机，再建立一层子文件夹。
  2. 每个主机子文件夹下，存放CubeFS的Json配置文件。（Docker部署文件不生成，因为所有部署所需的信息都能直接从集群拓扑文件中生成）
  3. 真正执行时。使用 sftp 自动将 Json 拷到对应的主机上，再使用 Docker SDK 创建容器并启动服务。
4. 可能还会存一些元数据相关的信息，比如每个服务对应的容器id等。（但其实必要性没那么大，可以通过确定的规则来生成容器名，比如 <角色组-序号>）

> 注：
> 1. 本来想选用 docker-compose，这样一个主机一个 compose 就好，不过发现 docker/compose 库对 docker compose 的支持不是很好，compose 的应用场景更多的是在 CLI 环境下，并且传入配置文件解析成结构体并分析其中的容器信息。而本项目本身就能从集群拓扑文件中生成部署所需的所有信息，所以没有必要用 docker compose。
> 2. 后续如果要支持多集群，可以给在集群拓扑文件中加一个集群名作为前缀，自动附到每个容器前面。

## 5. 节点扩容、缩容、服务重启、服务升级

这块的复杂之处在于，对于每个节点上的某个服务的操作有多种（创建、升级、修改等），同时一个服务的操作可能会影响到其他服务（比如 master 新增一个，DataNode等类型的节点需要依次修改配置并重启）。

本项目设计了几个基本的概念，来更为结构化地去解决这个问题：

* **原子操作**：
  * 对某个节点上的某个特定服务（容器）进行的特定操作。
  * 原子操作目前只有 **创建、删除、修改并重启** 三种。
* **角色组操作**：
  * 对某个角色组的所有实例进行的操作。例如 DataNode 分别在 5 个机器上共部署了 10 个实例，那么这些实例组成了一个 DataNode 角色组。
  * 角色组的操作，本质上是对每个实例进行原子操作的按序执行。
* **影响传递**:
  * 某个角色组造成的影响，会传递到其他角色组。例如 master 组发生了扩容、缩容操作，会影响到几乎其他所有角色组的变更。（master修改并重启并不会影响其他角色组）
  * 通过类似 DAG（有向无环图）的方式，来描述影响传递的关系。描述方式大概是：「A角色组的扩容、缩容操作 -> B角色组的修改并重启操作」
  * 影响图中，并不需要定义如何去影响，只需要起到告知被影响方需要做出响应的动作即可。因为在用户执行 apply 命令时，我们已经能根据最新的集群拓扑文件，知道每个角色组有哪些实例，以及每个实例的最新的配置文件。
  * A角色组发生的变化，只会通知到整个B角色组（不会直接通知到实例），由角色组内部负责对每个实例进行必要的原子操作。

直接说概念有些抽象，我们举几个简单的例子：

1. 用户想缩减 DataNode 的实例

* 用户需要做：
  1. 修改集群拓扑文件，删除了 DataNode 在 C 机器上的实例。
  2. 执行 apply 命令，等待执行完成。
* 程序需要做：
  1. 根据最新的集群拓扑文件，算出最新的部署文件清单（每个角色组在机器列表中的部署情况，生成每个实例的最新的配置文件）
  2. 对比上一次部署的部署文件清单，算出改变量。（这个例子中，发现只有 DataNode 角色组发生了改变，且只有 C 机器上的 DataNode 实例被删除了）
  3. 对 C 机器上的 DataNode 实例进行原子操作，即删除容器。
  4. 读取影响图，发现 DataNode 角色组的缩容操作不会影响到其他角色组，所以不需要做其他操作。
  5. 完成

2. 用户想扩容 Master

* 用户需要做：
  1. 修改集群拓扑文件，增加了 Master 在 D 机器上的实例。
  2. 执行 apply 命令，等待执行完成。
* 程序需要做：
  1. 根据最新的集群拓扑文件，算出最新的部署文件清单（每个角色组在机器列表中的部署情况，生成每个实例的最新的配置文件）
  2. 对比上一次部署的部署文件清单，算出改变量。（这个例子中，Master 角色组发生了改变，为 D 机器上的 Master 实例增加了一个。但同时，生成的最新配置文件中， DataNode 的配置文件也会发生变化，因为 DataNode 需要配置所有的 Master 节点地址）
  3. 程序识别到意图是对 Master 角色组进行扩容操作，而 Master 角色组扩容操作，拆分成原子操作为：
     1. 依次创建出新的 Master 实例，配置文件记录了所有的新的 Master 实例的地址。
     2. 依次修改旧的 Master 实例的配置文件（以添加新的 Master 实例的地址），并依次重启。
     3. 执行上述的对 Master 角色组的原子操作。
  4. 读取影响图，发现 Master 角色组的扩容操作会影响到其他角色组（DataNode、MetaNode组等）
     1. 这里以 DataNode 组为例。发现 Master 角色组扩容操作会触发 DataNode 组的修改并重启操作。
     2. 对 DataNode 组执行修改并重启操作，直接使用生成的最新的配置（即依次对组内的每个实例执行修改并重启操作）
     3. 读取影响图，发现 DataNode 组的修改并重启操作不会影响到其他角色组，所以不需要做其他操作。
  5. 完成

> 注：
> 1. 修改配置，修改镜像，本质上都属于「修改并重启」。
> 2. 在分析影响图并对被影响角色组执行操作完操作的时候，这里其实还会对已经执行完操作的组打上标记。这就像广度/深度优先遍历那样，目的都是为了遍历全图，但是每个点只会遍历一次。影响图也是类似，假如A角色组和B角色组的操作都会触发角色组C的操作，那么当A和B角色组都执行完操作后，才会执行C角色组的操作，避免重复操作。
> 3. 目前先考虑用户单次只主动改动一个角色组的情况，多个角色组的情况类似，只是需要继续
> 4. 暂不考虑某个角色组升级到一半中断的情况，内部先加入重试逻辑

## 6. 中间数据存储（是否需要数据库？）

## 其他细节


## 致谢

1. [curveadm](https://github.com/opencurve/curveadm) 是一个很成熟的 adm 工具，CuebFS Adm 在用户交互设计上参考了 curveadm 的很多思，感谢 curveadm 的前期探索。不过技术实现思路， cubefsadm 几乎完全不同，例如 Docker 控制方式与 YAML 解析方案。希望二者能够共同进步，为用户提供更好的体验。
2. ChatGPT。本次 POC 开发，ChatGPT 在快速生成初版代码上提供了很大的帮助，大大加快了开发进度。
