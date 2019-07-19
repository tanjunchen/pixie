#include <gtest/gtest.h>

#include <string>
#include <thread>
#include <vector>

#include "src/common/base/base.h"
#include "src/stirling/testing/tcp_socket.h"

namespace pl {
namespace stirling {
namespace testing {

TEST(TCPSocketTest, DataIsWrittenAndReceivedCorrectly) {
  TCPSocket server;
  server.Bind();

  std::vector<std::string> received_data;
  TCPSocket client;
  std::thread client_thread([&server, &client, &received_data]() {
    client.Connect(server);
    std::string data;
    while (client.Read(&data)) {
      received_data.push_back(data);
    }
  });
  server.Accept();
  EXPECT_EQ(2, server.Write("a,"));
  EXPECT_EQ(3, server.Send("bc,"));
  EXPECT_EQ(4, server.Send("END,"));
  EXPECT_EQ(7, server.SendMsg({"send", "msg"}));

  server.Close();
  client_thread.join();
  // read() might get all data from multiple write() because of kernel buffering, so we can only
  // check the concatenated string.
  EXPECT_EQ("a,bc,END,sendmsg", absl::StrJoin(received_data, ""));
}

TEST(TCPSocketTest, SendMsgAndRecvMsg) {
  TCPSocket server;
  server.Bind();

  std::vector<std::string> received_data;
  TCPSocket client;
  std::thread client_thread([&server, &client, &received_data]() {
    client.Connect(server);
    while (client.RecvMsg(&received_data) > 0) {
    }
  });
  server.Accept();
  EXPECT_EQ(14, server.SendMsg({"sendmsg", "recvmsg"}));

  server.Close();
  client_thread.join();

  EXPECT_EQ("sendmsgrecvmsg", absl::StrJoin(received_data, ""));
}

}  // namespace testing
}  // namespace stirling
}  // namespace pl
