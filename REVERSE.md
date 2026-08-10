# 门禁设备 HTTP 主动上传协议说明文档

## 1. 接口概述

门禁设备在发生门状态变更或人员通行时，会主动向指定的 Web 后端（IP:PORT）发起 **HTTP POST** 请求，数据格式为 JSON（含抓拍照时可能为 `multipart/form-data`）。

* **传输协议**：HTTP / HTTPS
* **请求方式**：POST
* **字符编码**：UTF-8

---

## 2. 数据结构定义

顶层结构包含 `Events` 数组（离线补传或批量事件）或单独的事件对象。

### 2.1 顶层通用字段

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| `Action` | String | 事件动作，如 `"Pulse"`（实时脉冲推送） |
| `Code` | String | 事件分类代码，决定 `Data` 内的具体数据结构（`"AccessControl"` / `"DoorStatus"`） |
| `Index` | Integer | 门禁通道/读卡器编号（`0` 代表 1 号门/通道） |
| `DataSource` | String | 数据来源（`"Offline"` 离线补传，`"Realtime"` 实时） |
| `Data` | Object | 事件数据载荷（详见下文 2.2 与 2.3） |

---

### 2.2 `Code: "AccessControl"`（通行/刷卡/人脸记录）

当有人员刷卡、人脸识别或按下出门按钮时触发该事件。

| 字段名 | 类型 | 示例值 | 说明 |
| --- | --- | --- | --- |
| **`UserID`** | String | `"REDACTED_USER_ID"` | **用户 ID/编号**（按钮开门时为空 `""`） |
| **`CardName`** | String | `"REDACTED_NAME"` | **通行人员姓名**（注意 UTF-8 编码；按钮开门时为空 `""`） |
| **`CardNo`** | String | `""` | IC 卡号（刷卡开门时填充，人脸/按钮时为空 `""`） |
| **`Method`** | Integer | `15` | **通行验证方式**：<br>

<br>• `1`：刷卡<br>

<br>• `2`：密码<br>

<br>• `3`：人脸识别<br>

<br>• `5`：**出门按钮**<br>

<br>• `15`：**人脸识别开门** |
| **`Type`** | String | `"Entry"` | **通行方向**：`"Entry"`（进门） / `"Exit"`（出门） |
| **`Status`** | Integer | `1` | **验证结果**：`1`（通过/成功开门） / `0`（拒绝/失败） |
| **`SN`** | String | `"REDACTED_DEVICE_SN"` | 设备序列号 |
| **`CreateTime`** | Long | `1700000000` | 事件发生时间的 Unix 时间戳（秒） |
| **`RealUTC`** / **`UTC`** | Long | `1700000000` | 标准时间戳（秒） |
| **`Name`** | String | `"门1"` | 门名称 |
| **`ReaderID`** | String | `"1"` | 读卡器/识别头 ID |
| **`Druss`** | Boolean | `false` | 是否为胁迫报警（`true` 表示被胁迫开门） |
| **`ImageInfo`** | Array | `[...]` | 抓拍照图像元数据列表（长/宽/偏移量） |
| **`ErrorCode`** | Integer | `0` | 错误码（`0` 表示正常） |
| **`BlockId`** | Integer | `10001` | 事件记录自增 Block ID |

---

### 2.3 `Code: "DoorStatus"`（门状态变更）

当物理锁具或门磁传感器检测到门打开或关上时触发。

| 字段名 | 类型 | 示例值 | 说明 |
| --- | --- | --- | --- |
| **`SN`** | String | `"REDACTED_DEVICE_SN"` | 设备序列号 |
| **`Status`** | String | `"Open"` | **门状态**：`"Open"`（开启） / `"Close"`（关闭） |
| **`UTC`** | Long | `1700000120` | 状态变更的时间戳（秒） |
| **`RealUTC`** | Long | `1700000120` | 实时 UTC 时间戳（秒） |

---

## 3. 示例 Data 报文

### 3.1 人脸识别进门事件（带身份）

```json
{
  "Action": "Pulse",
  "Code": "AccessControl",
  "DataSource": "Offline",
  "Data": {
    "UserID": "REDACTED_USER_ID",
    "CardName": "REDACTED_NAME",
    "Method": 15,
    "Type": "Entry",
    "Status": 1,
    "SN": "REDACTED_DEVICE_SN",
    "CreateTime": 1700000000,
    "ImageInfo": [
      { "Height": 640, "Width": 384, "Length": 15344, "Offset": 0 }
    ]
  }
}

```

### 3.2 出门按钮开门事件（无身份信息）

```json
{
  "Action": "Pulse",
  "Code": "AccessControl",
  "Data": {
    "UserID": "",
    "CardName": "",
    "Method": 5,
    "Type": "Exit",
    "Status": 1,
    "SN": "REDACTED_DEVICE_SN",
    "CreateTime": 1700000120
  }
}

```

---

## 4. 后端响应要求

设备发送 POST 数据包后，后端必须在 **3 秒内** 返回如下 JSON 响应，否则设备将判定为网络超时并重试或触发离线存盘：

* **HTTP Status Code**: `200 OK`
* **Response Body**:
```json
{
  "code": 0,
  "message": "success"
}

```
