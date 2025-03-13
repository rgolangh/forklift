package vantara

type Logins struct {
	HostGroupId string `json:"hostGroupId"`
	Islogin     string `json:"isLogin"`
	LoginWWN    string `json:"loginWwn"`
	WWNNickName string `json:"wwnNickName"`
}

type DataEntry struct {
	PortID string   `json:"portId"`
	WWN    string   `json:"wwn"`
	Logins []Logins `json:"logins"`
}

type JSONData struct {
	Data []DataEntry `json:"data"`
}

func FindHostGroupIDs(jsonData JSONData, loginWWNs []string) []Logins {
	results := []Logins{}
	for _, entry := range jsonData.Data {
		for _, login := range entry.Logins {
			for _, wwn := range loginWWNs {
				if login.LoginWWN == wwn {
					output := Logins{
						HostGroupId: login.HostGroupId,
						Islogin:     login.Islogin,
						LoginWWN:    login.LoginWWN,
						WWNNickName: login.WWNNickName,
					}
					results = append(results, output)
				}
			}
		}
	}
	return results
}
