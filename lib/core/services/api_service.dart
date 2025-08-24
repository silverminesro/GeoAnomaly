import 'dart:convert';
import 'package:http/http.dart' as http;
import '../constants.dart';

class ApiService {
  String? _token;

  void setToken(String token) {
    _token = token;
  }

  Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (_token != null) 'Authorization': 'Bearer $_token',
      };

  Future<Map<String, dynamic>> login(String username, String password) async {
    final response = await http.post(
      Uri.parse('${AppConstants.apiBaseUrl}/auth/login'),
      headers: _headers,
      body: jsonEncode({
        'username': username,
        'password': password,
      }),
    );

    if (response.statusCode == 200) {
      final data = jsonDecode(response.body);
      _token = data['token'];
      return data;
    } else {
      throw Exception('Failed to login: ${response.body}');
    }
  }

  Future<Map<String, dynamic>> getUserProfile() async {
    final response = await http.get(
      Uri.parse('${AppConstants.apiBaseUrl}/user/profile'),
      headers: _headers,
    );

    if (response.statusCode == 200) {
      return jsonDecode(response.body);
    } else {
      throw Exception('Failed to get user profile: ${response.body}');
    }
  }

  Future<Map<String, dynamic>> getInventory({
    int page = 1,
    int limit = 50,
    String? type,
  }) async {
    final queryParams = {
      'page': page.toString(),
      'limit': limit.toString(),
      if (type != null) 'type': type,
    };

    final uri = Uri.parse('${AppConstants.apiBaseUrl}/user/inventory').replace(
      queryParameters: queryParams,
    );

    final response = await http.get(uri, headers: _headers);

    if (response.statusCode == 200) {
      return jsonDecode(response.body);
    } else {
      throw Exception('Failed to get inventory: ${response.body}');
    }
  }

  Future<Map<String, dynamic>> getLevelDefinitions() async {
    final response = await http.get(
      Uri.parse('${AppConstants.apiBaseUrl}/user/levels'),
      headers: _headers,
    );

    if (response.statusCode == 200) {
      return jsonDecode(response.body);
    } else {
      throw Exception('Failed to get level definitions: ${response.body}');
    }
  }

  String getImageUrl(String itemType, String itemId) {
    return AppConstants.getImageUrl(itemType, itemId);
  }
}