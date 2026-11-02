import 'package:flutter/foundation.dart';

class DemoAuthSession extends ChangeNotifier {
  static const demoCode = '123456';

  String? _phoneNumber;
  bool _isAuthenticated = false;

  String? get phoneNumber => _phoneNumber;
  bool get isAuthenticated => _isAuthenticated;

  void setPhoneNumber(String phoneNumber) {
    _phoneNumber = phoneNumber;
  }

  bool verifyCode(String code) {
    if (_phoneNumber == null || code != demoCode) return false;
    _isAuthenticated = true;
    notifyListeners();
    return true;
  }

  void signOut() {
    _isAuthenticated = false;
    _phoneNumber = null;
    notifyListeners();
  }
}
