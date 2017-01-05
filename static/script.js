// goCode returns string of Go code extracted from #go div.
function goCode() {
  var code='';
  $.each($('#go pre'), function(i, val) {
    code += val.innerText + '\n';
  });
  return code;
}
function migoCode() {
  var code='';
  $.each($('#out pre'), function(i, val) {
    code += val.innerText + '\n';
  });
  return code;
}
// writeCode puts s into the #out div.
function writeTo(s, selector) {
  $(selector).empty();
  var strs = s.split('\n');
  for (var i=0; i<strs.length; i++) {
    $(selector).append($('<pre/>').html(strs[i]+'\n'));
  }
}
// reportTime puts t to the time div.
function reportTime(t) {
  if (t!=undefined && t!=null && t!='') {
    $('#time').html('Last operation completed in '+t);
  } else {
    $('#time').html('');
  }
}
(function(){
$('#ssa').on('click', function() {
  reportTime('');
  $.ajax({
    url: '/ssa',
    type: 'POST',
    data: goCode(),
    async: true,
    success: function(msg) {
      writeTo(msg, '#out');
      $('#out').attr('lang', 'Go SSA')
    }
  });
});
$('#cfsm').on('click', function() {
  reportTime('');
  $.ajax({
    url: '/cfsm',
    type: 'POST',
    data: goCode(),
    async: true,
    success: function(msg) {
      var obj=JSON.parse(msg);
      if (obj!=null && obj.CFSM!=null) {
        writeTo(obj.CFSM, '#out');
        reportTime(obj.time);
        $('#out').attr('lang', 'CFSM');
      } else {
        writeTo("JSON error", '#out');
      }
    }
  });
});
$('#migo').on('click', function() {
  reportTime('');
  $.ajax({
    url: '/migo',
    type: 'POST',
    data: goCode(),
    async: true,
    success: function(msg) {
      var obj=JSON.parse(msg);
      if (obj!=null && obj.MiGo!=null) {
        writeTo(obj.MiGo, '#out');
        reportTime(obj.time);
        $('#out').attr('lang', 'MiGo');
      } else {
        writeTo("JSON error", '#out');
      }
    }
  });
});
$('#example').on('click', function() {
  reportTime('');
  $.ajax({
    url: '/load',
    type: 'POST',
    data: $('#examples option:selected').text(),
    async: true,
    success: function(msg) {
      writeTo(msg, '#go');
      writeTo('No output.', '#out');
      $('#out').removeAttr('lang');
    }
  });
});
$('#gong').on('click', function() {
  if ($('#out').attr('lang') != 'MiGo') {
    return false
  }
  reportTime('');
  $.ajax({
    url: '/gong',
    type: 'POST',
    data: migoCode(),
    async: true,
    success: function(msg) {
      var obj = JSON.parse(msg);
      if (obj!=null&&obj.Gong!=null) {
        writeTo(obj.Gong, '#gong-output');
        reportTime(obj.time);
        $('#gong-wrap').addClass('visible');
      } else {
        writeTo("JSON error", '#gong-output');
      }
    }
  });
});
$('#gong-output-close').on('click', function() {
  $('#gong-wrap').removeClass('visible');
})
$('#synthesis').on('click', function() {
  if ($('#out').attr('lang') != 'CFSM') {
    return false
  }
  reportTime('');
  $.ajax({
    url: '/synthesis?chan='+$('#chan-cfsm').val(),
    type: 'POST',
    data: migoCode(),
    async: true,
    success: function(msg) {
      var obj = JSON.parse(msg);
      if (obj!=null&&obj.SMC!=null) {
        writeTo(obj.SMC, '#synthesis-output');
        $('#synthesis-global').html(obj.Global)
        $('#synthesis-machines').html(obj.Machines)
        reportTime(obj.time);
        $('#synthesis-wrap').addClass('visible');
      } else {
        writeTo("JSON error", '#synthesis-output');
      }
    }
  });
});
$('#synthesis-output-close').on('click', function() {
  $('#synthesis-wrap').removeClass('visible');
})
writeTo('// Write Go code here\n'
  + 'package main\n\n'
  + 'import "fmt"\n\n'
  + 'func main() {\n'
  + '    ch := make(chan int)   // Create <b>channel</b> <i>ch</i>\n'
  + '    go func(ch chan int) { // Spawn <b>goroutine</b>\n'
  + '        ch <- 42           // <b>Send</b> value to <i>ch</i>\n'
  + '    }(ch)\n'
  + '    fmt.Println(<-ch)      // <b>Recv</b> value from <i>ch</i>\n'
  + '}\n', '#go');
})()
