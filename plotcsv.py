import numpy as np
import pandas as pd

import csv
import json
import os
from matplotlib import pyplot as plt


def plotcsv(csvName, pcolor='red', plabel='GA'):
    # x = np.linspace(0, 30, 31)
    with open(os.path.join("csv", csvName), 'r', encoding='utf-8') as file:
        # 1.创建阅读器对象
        reader = csv.reader(file)
        # 2.读取文件头信息
        header_row = next(reader)
        # for index, column_header in enumerate(header_row):
        #     print(index, column_header)

        round, udpnums = [], []
        for row in reader:
            round.append(int(row[0]))
            udpnums.append(float(row[1]) / 10000)

    plt.plot(round, udpnums, color=pcolor, label=plabel)


csv_dict = {'GA': 'GA_10000nodes.csv',
            'BEBG': 'BEBG_10000nodes.csv',
            'PBEBG': 'PBEBG_10000nodes.csv',
            'NBEBG': 'NBEBG_10000nodes.csv'
            }
colors = ['red', 'blue', 'green', 'gold', 'purple', 'brown']
it_colors = iter(colors)

plt.rcParams['font.sans-serif'] = ['SimHei']
fig = plt.figure(dpi=128, figsize=(8, 5))
# fig.autofmt_xdate()

for label, csvname in csv_dict.items():
    plotcsv(csvname, next(it_colors), label)

plt.title('数据增长情况', fontsize=16)
plt.xlabel('周期T', fontsize=16)
plt.ylabel('UDP数据包个数(*10^4)', fontsize=16)
plt.tick_params(axis='both', which='major', labelsize=16)
plt.legend(loc='best')  # 图列位置，可选best，center等
plt.xlim(14, 22)  # x轴坐标轴
plt.ylim((1, 10))  # y轴坐标轴
plt.show()

# config_file = open('config.json', 'r')
# text = config_file.read()
# config_file.close()
# config = json.loads(text)
# csvName = "test.csv"
# if 4 == config["gossip"]:
#     csvName = "GA_" + str(config["count"]) + "nodes.csv"
# elif 5 == config["gossip"]:
#     csvName = "BEB_" + str(config["count"]) + "nodes.csv"
# elif 6 == config["gossip"]:
#     csvName = "PBEB_" + str(config["count"]) + "nodes.csv"
# elif 7 == config["gossip"]:
#     csvName = "NBEB_" + str(config["count"]) + "nodes.csv"
# else:
#     csvName = "gossip_" + str(config["count"]) + "nodes.csv"
