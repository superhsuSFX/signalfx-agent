package hostid

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// AzureUniqueID constructs the unique ID of the underlying Azure VM.  If
// not running on Azure VM, returns the empty string.
// Details about Azure Instance Metadata ednpoint:
// https://docs.microsoft.com/en-us/azure/virtual-machines/windows/instance-metadata-service
func AzureUniqueID() string {
	c := http.Client{
		Timeout: 1 * time.Second,
	}
	req, err := http.NewRequest("GET", "http://169.254.169.254/metadata/instance?api-version=2018-10-01", nil)
	if err != nil {
		return ""
	}

	req.Header.Set("Metadata", "true")
	resp, err := c.Do(req)
	if err != nil {
		return ""
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	type Info struct {
		SubscriptionID    string `json:"subscriptionId"`
		ResourceGroupName string `json:"resourceGroupName"`
		Name              string `json:"name"`
		VMScaleSetName    string `json:"vmScaleSetName"`
	}

	var compute struct {
		Doc Info `json:"compute"`
	}

	err = json.Unmarshal(body, &compute)
	if err != nil {
		return ""
	}

	if compute.Doc.SubscriptionID == "" || compute.Doc.ResourceGroupName == "" || compute.Doc.Name == "" {
		return ""
	}

	if compute.Doc.VMScaleSetName == "" {
		return fmt.Sprintf("%s/%s/microsoft.compute/virtualmachines/%s", compute.Doc.SubscriptionID, compute.Doc.ResourceGroupName, compute.Doc.Name)
	}

	instanceID := strings.TrimLeft(compute.Doc.Name, compute.Doc.VMScaleSetName+"_")

	// names of VM's in VMScalesets seem to follow the of `<scale-set-name>_<instance-id>`
	// where scale-set-name is alphanumeric (and is the same as compute.vmScaleSetName
	// field from the metadata endpoint)
	if instanceID == "" {
		return ""
	}

	return fmt.Sprintf("%s/%s/microsoft.compute/virtualmachinescalesets/%s/virtualmachines/%s", compute.Doc.SubscriptionID, compute.Doc.ResourceGroupName, compute.Doc.VMScaleSetName, instanceID)

}
