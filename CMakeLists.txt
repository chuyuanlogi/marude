cmake_minimum_required(VERSION 3.16)

set(CMAKE_DISABLE_FIND_PACKAGE_MSVC TRUE)
set(CMAKE_C_COMPILER_WORKS TRUE)
set(CMAKE_CXX_COMPILER_WORKS TRUE)

project(marude NONE)

find_package(Git REQUIRED)
execute_process(
	COMMAND ${GIT_EXECUTABLE} describe
	OUTPUT_VARIABLE GIT_VERSION
	WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
	OUTPUT_STRIP_TRAILING_WHITESPACE
	ERROR_QUIET
)

if(NOT GIT_VERSION)
	set(GIT_VERSION "cmake-debug")
	message("cannot get git version!")
else()
	message("build version: ${GIT_VERSION}")
endif()

set(GO_BUILD_FLAGS_DEBUG -X main.Version=${GIT_VERSION})
set(GO_BUILD_FLAGS_RELEASE -X main.Version=${GIT_VERSION} -s -w)

if(WIN32)
	set(PROC_FILENAME "${CMAKE_CURRENT_LIST_DIR}/system/win")
elseif(LINUX)
	set(PROC_FILENAME "${CMAKE_CURRENT_LIST_DIR}/system/linux")
elseif(APPLE)
	set(PROC_FILENAME "${CMAKE_CURRENT_LIST_DIR}/system/macos")
else()
	set(PROC_FILENAME "${CMAKE_CURRENT_LIST_DIR}/system/linux")
endif()

set(CONF_SAMPLE "${CMAKE_CURRENT_LIST_DIR}/system/conf")

function(build_go NAME SRC_PATH EXTEND_FLAG)
	add_custom_command(
		OUTPUT .${NAME}.gocmd
		COMMAND cd ${SRC_PATH} && go mod tidy
		COMMAND ${CMAKE_COMMAND} -E make_directory ${CMAKE_CURRENT_BINARY_DIR}/output
		COMMAND go build -C ${SRC_PATH} -o "${CMAKE_CURRENT_BINARY_DIR}/output" -ldflags="${GO_BUILD_FLAGS_${CMAKE_BUILD_TYPE}}" ${EXTEND_FLAG}
		WORKING_DIRECTORY ${CMAKE_CURRENT_LIST_DIR}
		COMMAND ${CMAKE_COMMAND} -E copy_directory ${PROC_FILENAME} "${CMAKE_CURRENT_BINARY_DIR}/output"
		COMMAND ${CMAKE_COMMAND} -E copy_directory ${CONF_SAMPLE} "${CMAKE_CURRENT_BINARY_DIR}/output/conf"
		COMMENT "Building go project ... ${NAME}"
	)

	add_custom_target(${NAME} ALL DEPENDS .${NAME}.gocmd)
endfunction()

build_go(server "${CMAKE_CURRENT_LIST_DIR}/server" "")
build_go(client "${CMAKE_CURRENT_LIST_DIR}/client" "")