add_library(ciscoapps STATIC
        rmcat-receiver.cc
        rmcat-sender.cc
        rtp-header.cc
        )

target_include_directories(ciscoapps PUBLIC "${CMAKE_SOURCE_DIR}")

include_directories(${NS3_INSTALL_DIR}/include)