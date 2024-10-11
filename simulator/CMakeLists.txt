cmake_minimum_required(VERSION 3.10)
project(Simulator)

set(CMAKE_CXX_STANDARD 20)

add_subdirectory(cisco)

# Create a new CMake target for the executable
add_executable(simulator simulator.cpp)

target_link_libraries(simulator syncodecs)

file(GLOB NS3_LIBRARIES ${NS3_INSTALL_DIR}/lib/*)
include_directories(${NS3_INSTALL_DIR}/include)
target_link_libraries(simulator ${NS3_LIBRARIES})
