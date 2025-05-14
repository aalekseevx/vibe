## VIBE: VIdeoBitrateEstimator for pion

## Known issues

- FlexFEC not encoded correctly
- FlecFEC not decoded
- isRTX, isFEC flags are not reliable on the receiver side
- Nothing happens when bitrate > capacity 
- Vnet simulation not exiting correctly with synctest=true
- CSRC is used as a place to log

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
