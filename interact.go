package libv2ray

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nekohasekai/AndroidLibV2rayLite/VPN"
	mobasset "golang.org/x/mobile/asset"

	v2core "github.com/v2fly/v2ray-core/v4"
	v2filesystem "github.com/v2fly/v2ray-core/v4/common/platform/filesystem"
	v2stats "github.com/v2fly/v2ray-core/v4/features/stats"
	v2serial "github.com/v2fly/v2ray-core/v4/infra/conf/serial"
	_ "github.com/v2fly/v2ray-core/v4/main/distro/all"
	v2internet "github.com/v2fly/v2ray-core/v4/transport/internet"

	v2applog "github.com/v2fly/v2ray-core/v4/app/log"
	v2commlog "github.com/v2fly/v2ray-core/v4/common/log"
)

const (
	v2Asset = "v2ray.location.asset"
)

/*V2RayPoint V2Ray Point Server
This is territory of Go, so no getter and setters!
*/
type V2RayPoint struct {
	SupportSet   V2RayVPNServiceSupportsSet
	statsManager v2stats.Manager

	dialer    *VPN.ProtectedDialer
	v2rayOP   sync.Mutex
	closeChan chan struct{}

	Vpoint    *v2core.Instance
	IsRunning bool

	DomainName           string
	ConfigureFileContent string
}

/*V2RayVPNServiceSupportsSet To support Android VPN mode*/
type V2RayVPNServiceSupportsSet interface {
	Protect(int) bool
	OnEmitStatus(string)
}

/*RunLoop Run V2Ray main loop
 */
func (v *V2RayPoint) RunLoop(prefIPv6 bool) (err error) {
	v.v2rayOP.Lock()
	defer v.v2rayOP.Unlock()
	//Construct Context

	if !v.IsRunning {
		v.closeChan = make(chan struct{})
		v.dialer.PrepareResolveChan()
		go func() {
			select {
			// wait until resolved
			case <-v.dialer.ResolveChan():
				// shutdown VPNService if server name can not reolved
				if !v.dialer.IsVServerReady() {
					log.Println("vServer cannot resolved, shutdown")
					v.StopLoop()
					v.SupportSet.OnEmitStatus("Shutdown")
				}

			// stop waiting if manually closed
			case <-v.closeChan:
			}
		}()

		v.dialer.PrepareDomain(v.DomainName, v.closeChan, prefIPv6)

		err = v.pointloop()
	}
	return
}

/*StopLoop Stop V2Ray main loop
 */
func (v *V2RayPoint) StopLoop() (err error) {
	v.v2rayOP.Lock()
	defer v.v2rayOP.Unlock()
	if v.IsRunning {
		close(v.closeChan)
		v.shutdownInit()
		v.SupportSet.OnEmitStatus("Closed")
	}
	return
}

//Delegate Funcation
func (v V2RayPoint) QueryStats(tag string, direct string) int64 {
	if v.statsManager == nil {
		return 0
	}
	counter := v.statsManager.GetCounter(fmt.Sprintf("outbound>>>%s>>>traffic>>>%s", tag, direct))
	if counter == nil {
		return 0
	}
	return counter.Set(0)
}

func (v *V2RayPoint) shutdownInit() {
	v.IsRunning = false
	v.Vpoint.Close()
	v.Vpoint = nil
	v.statsManager = nil
}

func (v *V2RayPoint) pointloop() error {
	log.Println("loading v2ray config")
	config, err := v2serial.LoadJSONConfig(strings.NewReader(v.ConfigureFileContent))
	if err != nil {
		log.Println(err)
		return err
	}

	log.Println("new v2ray core")
	v.Vpoint, err = v2core.New(config)
	if err != nil {
		v.Vpoint = nil
		log.Println(err)
		return err
	}
	v.statsManager = v.Vpoint.GetFeature(v2stats.ManagerType()).(v2stats.Manager)

	log.Println("start v2ray core")
	v.IsRunning = true
	if err := v.Vpoint.Start(); err != nil {
		v.IsRunning = false
		log.Println(err)
		return err
	}

	v.SupportSet.OnEmitStatus("Running")
	return nil
}

// InitV2Env set v2 asset path
func SetAssetsPath(envPath string, assetsPath string) {
	//Initialize asset API, Since Raymond Will not let notify the asset location inside Process,
	//We need to set location outside V2Ray
	if len(envPath) > 0 {
		os.Setenv(v2Asset, envPath)
	}

	//Now we handle read, fallback to gomobile asset (apk assets)
	v2filesystem.NewFileReader = func(path string) (io.ReadCloser, error) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			_, file := filepath.Split(path)
			return mobasset.Open(assetsPath + file)
		}
		return os.Open(path)
	}
}

//Delegate Funcation
func TestConfig(ConfigureFileContent string) error {
	_, err := v2serial.LoadJSONConfig(strings.NewReader(ConfigureFileContent))
	return err
}

/*NewV2RayPoint new V2RayPoint*/
func NewV2RayPoint(s V2RayVPNServiceSupportsSet, adns bool) *V2RayPoint {
	// inject our own log writer
	v2applog.RegisterHandlerCreator(v2applog.LogType_Console,
		func(lt v2applog.LogType,
			options v2applog.HandlerCreatorOptions) (v2commlog.Handler, error) {
			return v2commlog.NewLogger(createStdoutLogWriter()), nil
		})

	dialer := VPN.NewPreotectedDialer(s)
	v2internet.UseAlternativeSystemDialer(dialer)
	return &V2RayPoint{
		SupportSet: s,
		dialer:     dialer,
	}
}

func GetVersion() string {
	return fmt.Sprintf("v%s", v2core.Version())
}
