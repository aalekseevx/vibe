add_library(cisco-rmcat STATIC
        apps/rmcat-receiver.cc
        apps/rmcat-sender.cc
        apps/rtp-header.cc
        congestion-control/dummy-controller.cc
        congestion-control/nada-controller.cc
        congestion-control/sender-based-controller.cc
        syncodecs/syncodecs.cc
        syncodecs/traces-reader.cc
        topo/topo.cc
#        topo/wifi-topo.cc Not working for now
        topo/wired-topo.cc
        )

target_include_directories(cisco-rmcat PUBLIC "${CMAKE_CURRENT_SOURCE_DIR}")
include_directories(${NS3_INSTALL_DIR}/include)
