module github.com/nekohasekai/AndroidLibV2rayLite

go 1.15

require (
	golang.org/x/mobile v0.0.0-20210220033013-bdb1ca9a1e08
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c
	v2ray.com/core v4.19.1+incompatible
)

replace v2ray.com/core => github.com/v2fly/v2ray-core v1.24.5-0.20210104111944-a6efb4d60b86
