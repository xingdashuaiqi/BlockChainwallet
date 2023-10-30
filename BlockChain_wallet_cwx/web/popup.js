
$(function () {
  $("#loadPK").click(function () {
    var privatekeyvalue = $("#inputPrivateKey").val()
    console.log(privatekeyvalue)
    let data = {
      sender_private_key: privatekeyvalue
    }
    $.ajax({
      url: "http://127.0.0.1:8080/walletByPrivatekey",
      type: "POST",
      contentType: "application/json",
      data: JSON.stringify(data),
      success: function (response) {
        $("#inputPrivateKey").val(response["private_key"]);
        $("#inputPublic").val(response["public_key"]);
        $("#inputAddress").val(response["blockchain_address"]);
        saveUserInfo()
        getUserAmount()
        console.info(response);
      },
      error: function (error) {
        console.error(error);
      },
    });
  })
  $("#randGe").click(function () {
    var privatekeyvalue = $("#inputPrivateKey").val()
    console.log(privatekeyvalue)
    $.ajax({
      url: "http://127.0.0.1:8080/wallet",
      type: "POST",
      success: function (response) {
        $("#inputPrivateKey").val(response["private_key"]);
        $("#inputPublic").val(response["public_key"]);
        $("#inputAddress").val(response["blockchain_address"]);
        saveUserInfo()
        getUserAmount()
        console.info(response);
      },
      error: function (error) {
        console.error(error);
      },
    });
  })
  $("button.transList").click(function () {
    storage.get('data', function (result) {
      // 检查是否存在之前保存的数据
      if (result.data) {
        let _postdata = {
          hash: result.data.address
        };
        $.ajax({
          url: "http://localhost:5000/transactions/user/hash",
          type: "POST",
          contentType: "application/json",
          data: JSON.stringify(_postdata),
          success: function (response) {
            var template = $("#transactionTemplate").html();
            var r = Mustache.render(template, {
              transactions: response["transactions"]
            })
            $("#transData").html(r)
            console.info(response);
          },
          error: function (error) {
            console.error(error);
          },
        });
      }
    });
  })
  $("#buttonSubmit").click(function () {
    let confirm_text = "确定要发送吗?";
    let confirm_result = confirm(confirm_text);
    if (confirm_result !== true) {
      alert("取消");
      return;
    }
    let transaction_data = {
      sender_private_key: $("#inputPrivateKey").val(),
      sender_blockchain_address: $("#inputAddress").val(),
      recipient_blockchain_address: $("#inputReceiveAddress").val(),
      sender_public_key: $("#inputPublic").val(),
      value: $("#inputAmount").val(),
    };
    $.ajax({
      url: "http://localhost:8080/transaction",
      type: "POST",
      contentType: "application/json",
      data: JSON.stringify(transaction_data),
      success: function (response) {
        console.info("response:", response);
        console.info("response.message:", response.message);
        if (response.message === "fail") {
          alert("failed 222");
          return;
        }
        alert("发送成功" + JSON.stringify(response));
      },
      error: function (response) {
        console.error(response);
        alert("发送失败");
      },
    });
  });
  $("#serachSubmit").click(function () {
    var inputSearchBlock = $("#inputSearchBlock").val()
    var inputSearchTran = $("#inputSearchTran").val()
    var isPureNumber = !isNaN(parseFloat(inputSearchBlock)) && isFinite(inputSearchBlock)
    if (inputSearchBlock == "" && inputSearchTran == "") {
      alert("请输入数据")
    }else if (inputSearchBlock != "" && inputSearchTran != "") {
      alert("只能同时查询一个")
      return
    } else if (inputSearchBlock != "") {
      if (!isPureNumber) {
        searchBlockByHash(inputSearchBlock);
      } else {
        searchBlockByNumber(inputSearchBlock);
      }
    } else if (inputSearchTran != "") {
      searchTransactionByHash(inputSearchTran);
    }
  });


})


var storage = chrome.storage.sync;
function saveUserInfo() {
  // 获取用户输入的数据
  var inputData = document.getElementById('inputPrivateKey').value;
  var publicKey = document.getElementById('inputPublic').value;
  var address = document.getElementById('inputAddress').value;
  let data = {
    privateKey: inputData,
    publicKey: publicKey,
    address: address,
  }
  // 将数据保存到存储中
  storage.set({ 'data': data }, function () {
    console.log('数据已保存');
  });
};

document.addEventListener('DOMContentLoaded', function () {
  // 从存储中检索数据
  getUserData()
  getUserAmount()
});

function getUserData() {
  storage.get('data', function (result) {
    // 检查是否存在之前保存的数据
    if (result.data) {
      // 将数据应用到用户界面
      document.getElementById('inputPrivateKey').value = result.data.privateKey;
      document.getElementById('inputPublic').value = result.data.publicKey;
      document.getElementById('inputAddress').value = result.data.address;
    }
  });
}

function searchBlockByHash(inputHash) {
  let _postdata = {
    hash: inputHash
  };
  $.ajax({
    url: "http://127.0.0.1:5000/block/hash",
    type: "POST",
    contentType: "application/json",
    data: JSON.stringify(_postdata),
    success: function (response) {
      var template = $("#blockTemplate").html();
      var r = Mustache.render(template, {
        timestamp: response["timestamp"],
        nonce: response["nonce"],
        hash: response["hash"],
        number: response["number"],
        previous_hash: response["previous_hash"],
        transactions: response["transactions"]
      })
      $("#searchResult").html(r)
      console.info(response);
    },
    error: function (error) {
      console.error(error);
    },
  });
}

function searchBlockByNumber(inputNumber) {
  let _postdata = {
    number: inputNumber
  };
  $.ajax({
    url: "http://127.0.0.1:5000/block/number",
    type: "POST",
    contentType: "application/json",
    data: JSON.stringify(_postdata),
    success: function (response) {
      var template = $("#blockTemplate").html();
      var r = Mustache.render(template, {
        timestamp: response["timestamp"],
        nonce: response["nonce"],
        hash: response["hash"],
        number: response["number"],
        previous_hash: response["previous_hash"],
        transactions: response["transactions"]
      })
      $("#searchResult").html(r)
      console.info(response);
    },
    error: function (error) {
      console.error(error);
    },
  });
}

function searchTransactionByHash(inputHash) {
  let _postdata = {
    hash: inputHash
  };
  $.ajax({
    url: "http://127.0.0.1:5000/transactions/hash",
    type: "POST",
    contentType: "application/json",
    data: JSON.stringify(_postdata),
    success: function (response) {
      var template = $("#transactionTemplate").html();
      var r = Mustache.render(template, {
        transactions: [response]
      })
      $("#searchResult").html(r)
      console.info(response);
    },
    error: function (error) {
      console.error(error);
    },
  });
}

function getUserAmount() {
  storage.get('data', function (result) {
    // 检查是否存在之前保存的数据
    if (result.data) {
      let _postdata = {
        blockchain_address: result.data.address
      };
      console.log("blockaddress:", JSON.stringify(_postdata));
      $.ajax({
        url: "http://127.0.0.1:5000/amount",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify(_postdata),
        success: function (response) {
          $("#wallet_amount").text(response["amount"]);
          console.info(response);
        },
        error: function (error) {
          console.error(error);
        },
      });
    }
  });
}
