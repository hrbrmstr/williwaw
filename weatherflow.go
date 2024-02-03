package main

type ObsSt struct {
	SerialNumber     string      `json:"serial_number"`
	Type             string      `json:"type"`
	HubSn            string      `json:"hub_sn"`
	Obs              [][]float64 `json:"obs"`
	FirmwareRevision int64       `json:"firmware_revision"`
}

type HubStatus struct {
	SerialNumber     string  `json:"serial_number"`
	Type             string  `json:"type"`
	FirmwareRevision string  `json:"firmware_revision"`
	Uptime           int64   `json:"uptime"`
	Rssi             int64   `json:"rssi"`
	Timestamp        int64   `json:"timestamp"`
	ResetFlags       string  `json:"reset_flags"`
	Seq              int64   `json:"seq"`
	FS               []int64 `json:"fs"`
	RadioStats       []int64 `json:"radio_stats"`
	MqttStats        []int64 `json:"mqtt_stats"`
}
