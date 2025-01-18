## VIBE: VIdeoBitrateEstimator for pion

### Also check out related [bwe-test repo](https://github.com/pion/bwe-test)

We aim at creating pluggable set of interceptors that work with [pion](https://github.com/pion/webrtc) for accurate 
bandwith estimation and simulcast resolving in dynamic network conditions.

Also, there are some experiments with ns3 + webrtc

### How to build the simulator:

1. Init submodules: `git submodule init --recursive`
2. Build WebRTC outside the repo using the [manual](https://webrtc.googlesource.com/src/+/refs/heads/main/docs/native-code/development/#building)
3. Clone and build ns3 outside the repo using the [manual](https://gitlab.com/nsnam/ns-3-dev#building-ns-3)
4. Compile simulator using CMake with options `-DNS3_INSTALL_DIR=/path/to/ns3/build  -DWEBRTC_SRC_DIR=/path/to/webrtc/src -DWEBRTC_INSTALL_DIR=/path/to/webrtc/src/out/Default `

### Credits and related project:

- [Pion](https://pion.ly/)
- [WebRTC](http://webrtc.org/)
- [SoonyangZhang/rmcat-ns3](https://github.com/SoonyangZhang/rmcat-ns3)
- [Razor](https://github.com/yuanrongxi/razor)
- [Cisco/ns3-rmcat](https://github.com/cisco/ns3-rmcat) 
- [Cisco/syncodecs](https://github.com/cisco/syncodecs)

### Credits
- [Sean-Der](https://github.com/Sean-Der) 
- [Cisco](https://github.com/cisco/)

### License

All new code is licensed under the MIT License. This project also utilizes Cisco libraries, which are licensed under the Apache License 2.0.
