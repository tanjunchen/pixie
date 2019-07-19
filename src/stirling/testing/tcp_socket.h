#pragma once

#include <netinet/in.h>

#include <string>
#include <string_view>
#include <vector>

namespace pl {
namespace stirling {
namespace testing {

/**
 * @brief A simple wrapper of the syscalls for IPv4 TCP socket.
 */
class TCPSocket {
 public:
  TCPSocket();
  ~TCPSocket();
  void Bind();
  void Accept();
  void Close();
  ssize_t Write(std::string_view data) const;
  ssize_t Send(std::string_view data) const;
  ssize_t SendMsg(const std::vector<std::string_view>& data) const;
  void Connect(const TCPSocket& addr);
  bool Read(std::string* data);
  bool Recv(std::string* data);
  ssize_t RecvMsg(std::vector<std::string>* data) const;
  int sockfd() const { return sockfd_; }

 private:
  bool closed = false;
  int sockfd_;
  struct sockaddr_in addr_;
  static constexpr int kBufSize = 128;
  char buf_[kBufSize];
};

}  // namespace testing
}  // namespace stirling
}  // namespace pl
