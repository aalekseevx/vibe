add_executable(cisco-example rmcat-example.cc)

include_directories(${NS3_INSTALL_DIR}/include)
target_link_libraries(cisco-example cisco-rmcat)
file(GLOB NS3_LIBRARIES ${NS3_INSTALL_DIR}/lib/*)
target_link_libraries(cisco-example ${NS3_LIBRARIES})
