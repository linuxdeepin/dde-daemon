package scale

import (
	"fmt"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"math"
	"os"
	"path/filepath"
)

type monitorSizeInfo struct {
	width, height     uint16
	mmWidth, mmHeight uint32
}

func HasRandr1d2(x *x.Conn) bool {
	randrVersion, err := randr.QueryVersion(x, randr.MajorVersion, randr.MinorVersion).Reply(x)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("randr version %d.%d", randrVersion.ServerMajorVersion, randrVersion.ServerMinorVersion)
		if randrVersion.ServerMajorVersion > 1 ||
			(randrVersion.ServerMajorVersion == 1 && randrVersion.ServerMinorVersion >= 2) {
			return true
		}
	}
	return false
}

func GetRecommendedScaleFactor(x *x.Conn) float64 {
	if !HasRandr1d2(x) {
		return 1.0
	}
	resources, err := getScreenResources(x)
	if err != nil {
		return 1
	}
	cfgTs := resources.ConfigTimestamp

	var monitors []*monitorSizeInfo
	for _, output := range resources.Outputs {
		outputInfo, err := randr.GetOutputInfo(x, output, cfgTs).Reply(x)
		if err != nil {
			fmt.Printf("get output %v info failed: %v", output, err)
			return 1.0
		}
		if outputInfo.Connection != randr.ConnectionConnected {
			continue
		}

		if outputInfo.Crtc == 0 {
			continue
		}

		crtcInfo, err := randr.GetCrtcInfo(x, outputInfo.Crtc, cfgTs).Reply(x)
		if err != nil {
			fmt.Printf("get crtc %v info failed: %v", outputInfo.Crtc, err)
			return 1.0
		}
		monitors = append(monitors, &monitorSizeInfo{
			mmWidth:  outputInfo.MmWidth,
			mmHeight: outputInfo.MmHeight,
			width:    crtcInfo.Width,
			height:   crtcInfo.Height,
		})
	}

	if len(monitors) == 0 {
		return 1.0
	}

	// 允许用户通过 force-scale-factor.ini 强制设置全局缩放
	forceScaleFactor, err := GetForceScaleFactor()
	if err == nil {
		return forceScaleFactor
	}

	minScaleFactor := 3.0
	for _, monitor := range monitors {
		scaleFactor := calcRecommendedScaleFactor(float64(monitor.width), float64(monitor.height),
			float64(monitor.mmWidth), float64(monitor.mmHeight))
		if minScaleFactor > scaleFactor {
			minScaleFactor = scaleFactor
		}
	}
	return minScaleFactor
}

func getScreenResources(xConn *x.Conn) (*randr.GetScreenResourcesReply, error) {
	root := xConn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResources(xConn, root).Reply(xConn)
	return resources, err
}

func getForceScaleFactorFile() string {
	return filepath.Join(basedir.GetUserConfigDir(), "deepin/force-scale-factor.ini")
}

// GetForceScaleFactor 允许用户通过 force-scale-factor.ini 强制设置全局缩放
func GetForceScaleFactor() (float64, error) {
	fileName := getForceScaleFactorFile()
	_, err := os.Stat(fileName)
	if err == nil {
		kf := keyfile.NewKeyFile()
		err := kf.LoadFromFile(fileName)
		if err != nil && !os.IsNotExist(err) {
			fmt.Println("failed to load force-scale-factor.ini:", err)
		} else {
			forceScaleFactor, err := kf.GetFloat64("ForceScaleFactor", "scale")
			if err == nil && forceScaleFactor >= 1.0 && forceScaleFactor <= 3.0 {
				return forceScaleFactor, nil
			} else {
				fmt.Println("invalid forceScaleFactor:", forceScaleFactor, err)
			}
		}
	}
	return 1.0, fmt.Errorf("no valid force-scale-factor")
}

// calcRecommendedScaleFactor 计算推荐的缩放比
func calcRecommendedScaleFactor(widthPx, heightPx, widthMm, heightMm float64) float64 {
	if widthMm == 0 || heightMm == 0 {
		return 1
	}

	lenPx := math.Hypot(widthPx, heightPx)
	lenMm := math.Hypot(widthMm, heightMm)

	lenPxStd := math.Hypot(1920, 1080)
	lenMmStd := math.Hypot(477, 268)

	const a = 0.00158
	fix := (lenMm - lenMmStd) * (lenPx / lenPxStd) * a
	scaleFactor := (lenPx/lenMm)/(lenPxStd/lenMmStd) + fix

	return toListedScaleFactor(scaleFactor)
}

func toListedScaleFactor(s float64) float64 {
	const (
		min  = 1.0
		max  = 3.0
		step = 0.25
	)
	if s <= min {
		return min
	} else if s >= max {
		return max
	}

	for i := min; i <= max; i += step {
		if i > s {
			ii := i - step
			d1 := s - ii
			d2 := i - s

			if d1 >= d2 {
				return i
			} else {
				return ii
			}
		}
	}
	return max
}
