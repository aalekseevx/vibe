## VIBE: VIdeoBitrateEstimator for pion

We aim at creating pluggable set of interceptors that work with [pion](https://github.com/pion/webrtc) for accurate 
bandwith estimation and simulcast resolving in dynamic network conditions.

Also, there are some experiments with ns3 + webrtc

How to build the simulator:

1. Init submodules: `git submodule init --recursive`
2. Build WebRTC outside the repo using the [manual](https://webrtc.googlesource.com/src/+/refs/heads/main/docs/native-code/development/#building)
3. Clone and build ns3 outside the repo using the [manual](https://gitlab.com/nsnam/ns-3-dev#building-ns-3)
4. Compile simulator using CMake with options `-DNS3_INSTALL_DIR=/path/to/ns3/build  -DWEBRTC_SRC_DIR=/path/to/webrtc/src -DWEBRTC_INSTALL_DIR=/path/to/webrtc/src/out/Default `
