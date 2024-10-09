cmake_minimum_required(VERSION 3.10)
project(MyProject)

set(CMAKE_CXX_STANDARD 20)

# Define the NS3 directory variable
set(NS3_DIR /Users/alekseev-dev/vibe/ns3/build)

set(NS3_INCLUDES "${NS3_DIR}/include")

file(GLOB LIBRARIES ${NS3_DIR}/lib/*.dylib)
message("LIBRARIES = ${LIBRARIES}")

# Create a new CMake target for the executable
add_executable(simulator simulator.cpp)

include_directories(${NS3_INCLUDES})
link_libraries(${NS3_DIR}/lib)

target_link_libraries(simulator ${LIBRARIES})

